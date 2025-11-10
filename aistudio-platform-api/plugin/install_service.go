package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Kubernetes-api/helper"
	utils "Kubernetes-api/kubeutils"

	"github.com/gofiber/fiber/v2/log"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/yaml"
)

const (
	pluginsArtifactRoot = "artifacts/plugins"
	pluginsRegistryDir  = "artifacts/plugins/registry"
	defaultServicePort  = 80
	backendServicePort  = 8080
)

type operationKind string

const (
	operationInstall operationKind = "install"
	operationUpdate  operationKind = "update"
)

// Installer orchestrates the plugin workflow – download, validation,
// provisioning, signalling, and rollback.
type Installer struct {
	kube       *utils.KubernetesConfig
	httpClient *http.Client
}

func NewInstaller() *Installer {
	return &Installer{
		kube:       utils.NewKubernetesConfig(),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Install orchestrates a full plugin installation.
func (i *Installer) Install(ctx context.Context, identifier string, req PluginInstallRequest) (*PluginInstallResponse, error) {
	return i.execute(ctx, identifier, operationInstall, req)
}

// Update reuses the same workflow as Install but marks the operation as update.
func (i *Installer) Update(ctx context.Context, identifier string, req PluginUpdateRequest) (*PluginInstallResponse, error) {
	return i.execute(ctx, identifier, operationUpdate, req.PluginInstallRequest)
}

func (i *Installer) execute(ctx context.Context, identifier string, kind operationKind, req PluginInstallRequest) (*PluginInstallResponse, error) {
	if strings.TrimSpace(identifier) == "" {
		return nil, fmt.Errorf("identifier is required")
	}

	op, err := newInstallOperation(ctx, i, identifier, kind, req)
	if err != nil {
		return nil, err
	}
	defer op.cleanup()

	if err := op.run(ctx); err != nil {
		if rbErr := op.rollback(ctx); rbErr != nil {
			log.Errorf("failed to rollback plugin operation %s: %v", identifier, rbErr)
		}
		return nil, err
	}

	if err := op.finalize(ctx); err != nil {
		return nil, err
	}

	return op.response(), nil
}

// Rollback removes plugin resources and cleans up local metadata.
func (i *Installer) Rollback(ctx context.Context, identifier string, req PluginRollbackRequest) error {
	record, err := loadPluginRecord(identifier)
	if err != nil {
		return err
	}

	namespace := record.Namespace
	if namespace == "" {
		namespace = pluginNamespace
	}

	frontendName := fmt.Sprintf("%s-frontend", identifier)
	backendName := fmt.Sprintf("%s-backend", identifier)

	i.kube.DeleteDeployment(namespace, frontendName)
	i.kube.DeleteService(namespace, frontendName)
	if req.RoutePath != "" {
		frontendPath := fmt.Sprintf("/plugins/%s", req.RoutePath)
		if err := i.kube.DeleteRuleFromIngress(namespace, frontendPath, ingressName); err != nil {
			log.Errorf("failed to delete frontend ingress rule: %v", err)
		}
	}

	i.kube.DeleteDeployment(namespace, backendName)
	i.kube.DeleteService(namespace, backendName)
	if req.RoutePath != "" {
		backendPath := fmt.Sprintf("/plugins/%s/api", req.RoutePath)
		if err := i.kube.DeleteRuleFromIngress(namespace, backendPath, ingressName); err != nil {
			log.Errorf("failed to delete backend ingress rule: %v", err)
		}
	}

	if err := removeArtifacts(identifier); err != nil {
		return err
	}

	if err := deletePluginRecord(identifier); err != nil {
		return err
	}

	return nil
}

// -----------------------------------------------------------------------------
// installation operation
// -----------------------------------------------------------------------------

type installOperation struct {
	installer   *Installer
	identifier  string
	kind        operationKind
	req         PluginInstallRequest
	manifestRes *manifestParseResult

	artifactDir   string
	workspaceDir  string
	zipFile       string
	extractDir    string
	deploymentYml string
	logoPath      string
	dbCredentials *databaseCredentials
	namespace     string
	frontendURL   string
	backendURL    string

	rollbackStack []func(context.Context) error
	startTime     time.Time
}

func newInstallOperation(ctx context.Context, installer *Installer, identifier string, kind operationKind, req PluginInstallRequest) (*installOperation, error) {
	baseDir := filepath.Join(pluginsArtifactRoot, identifier)
	workDir := filepath.Join(baseDir, fmt.Sprintf(".tmp-%d", time.Now().UnixNano()))

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("prepare workspace: %w", err)
	}

	if err := os.MkdirAll(pluginsRegistryDir, 0o755); err != nil {
		return nil, fmt.Errorf("prepare registry: %w", err)
	}

	op := &installOperation{
		installer:     installer,
		identifier:    identifier,
		kind:          kind,
		req:           req,
		artifactDir:   baseDir,
		workspaceDir:  workDir,
		zipFile:       filepath.Join(workDir, "bundle.zip"),
		extractDir:    filepath.Join(workDir, "extracted"),
		deploymentYml: filepath.Join(baseDir, "deployment.yaml"),
		startTime:     time.Now(),
	}

	op.pushRollback(func(context.Context) error {
		return os.RemoveAll(workDir)
	})

	return op, nil
}

func (op *installOperation) run(ctx context.Context) error {
	if err := op.downloadZip(ctx); err != nil {
		return err
	}

	files, err := op.extractBundle()
	if err != nil {
		return err
	}

	if err := op.parseManifest(files); err != nil {
		return err
	}

	if err := op.ensureNamespace(ctx); err != nil {
		return err
	}

	if err := op.handleDatabase(ctx); err != nil {
		return err
	}

	if err := op.ensureDeploymentFile(); err != nil {
		return err
	}

	if err := op.applyDeployment(ctx); err != nil {
		return err
	}

	if err := op.ensureServicesAndIngress(); err != nil {
		return err
	}

	if err := op.handleLogo(ctx); err != nil {
		return err
	}

	if err := op.sendBackendSignals(ctx); err != nil {
		return err
	}

	if err := op.writeRegistryRecord(); err != nil {
		return err
	}

	return nil
}

func (op *installOperation) rollback(ctx context.Context) error {
	var firstErr error
	for _, fn := range op.rollbackStack {
		if err := safeCall(ctx, fn); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (op *installOperation) finalize(ctx context.Context) error {
	return safeCall(ctx, func(context.Context) error {
		return os.RemoveAll(op.workspaceDir)
	})
}

func (op *installOperation) response() *PluginInstallResponse {
	return &PluginInstallResponse{
		FrontendURL: op.frontendURL,
		BackendURL:  op.backendURL,
		Namespace:   op.namespace,
		Deployment:  op.deploymentYml,
		ReleaseID:   op.req.ReleaseID,
		EngineKey:   op.manifestRes.Manifest.EngineKey,
		Version:     op.manifestRes.Manifest.Version,
	}
}

func (op *installOperation) cleanup() {
	// best effort
	_ = os.RemoveAll(op.workspaceDir)
}

func (op *installOperation) pushRollback(fn func(context.Context) error) {
	op.rollbackStack = append([]func(context.Context) error{fn}, op.rollbackStack...)
}

func (op *installOperation) downloadZip(ctx context.Context) error {
	if op.req.ZipURL == "" {
		return fmt.Errorf("zipUrl is required")
	}
	log.Infof("downloading plugin bundle from %s", op.req.ZipURL)
	if err := helper.DownloadFile(op.req.ZipURL, op.zipFile); err != nil {
		return fmt.Errorf("download zip: %w", err)
	}
	op.pushRollback(func(context.Context) error {
		return os.Remove(op.zipFile)
	})
	return nil
}

func (op *installOperation) extractBundle() ([]string, error) {
	files, err := helper.ExtractZip(op.zipFile, op.extractDir)
	if err != nil {
		return nil, fmt.Errorf("extract bundle: %w", err)
	}
	op.pushRollback(func(context.Context) error {
		return os.RemoveAll(op.extractDir)
	})
	return files, nil
}

func (op *installOperation) parseManifest(files []string) error {
	res, err := parseManifestFromFiles(files)
	if err != nil {
		return err
	}
	if op.req.EngineKey != "" && !strings.EqualFold(op.req.EngineKey, res.Manifest.EngineKey) {
		return fmt.Errorf("manifest engine key %s does not match request %s", res.Manifest.EngineKey, op.req.EngineKey)
	}
	if !strings.EqualFold(op.identifier, res.Manifest.EngineKey) {
		return fmt.Errorf("identifier %s does not match manifest engine key %s", op.identifier, res.Manifest.EngineKey)
	}
	if op.req.ExpectedManifestSHA != "" && !strings.EqualFold(op.req.ExpectedManifestSHA, res.SHA256Hex) {
		return fmt.Errorf("manifest hash mismatch: expected %s got %s", op.req.ExpectedManifestSHA, res.SHA256Hex)
	}
	op.manifestRes = res
	if res.Manifest.Kubernetes.Namespace != "" {
		op.namespace = res.Manifest.Kubernetes.Namespace
	} else {
		op.namespace = pluginNamespace
	}
	return nil
}

func (op *installOperation) ensureNamespace(ctx context.Context) error {
	if strings.EqualFold(op.namespace, "") {
		op.namespace = pluginNamespace
	}
	// Ensure namespace exists; treat AlreadyExists as success.
	log.Infof("ensuring namespace %s", op.namespace)
	defer func() {
		// panic guard since CreateNamespace may panic
		if r := recover(); r != nil {
			log.Errorf("panic while ensuring namespace %s: %v", op.namespace, r)
		}
	}()
	op.installer.kube.CreateNamespace(op.namespace)
	return nil
}

func (op *installOperation) handleDatabase(ctx context.Context) error {
	mode := strings.ToLower(op.manifestRes.Manifest.Permissions.Database.Mode)
	if mode == "" || mode == "none" {
		return nil
	}
	creds, created, err := ensureDatabaseCredentials(op.artifactDir, op.identifier, op.manifestRes.Manifest)
	if err != nil {
		return err
	}
	op.dbCredentials = creds
	if created {
		op.pushRollback(func(context.Context) error {
			if err := rollbackDatabaseCredentials(op.artifactDir); err != nil {
				log.Errorf("rollback database credentials: %v", err)
				return err
			}
			return nil
		})
	}
	return nil
}

func (op *installOperation) ensureDeploymentFile() error {
	if err := os.MkdirAll(op.artifactDir, 0o755); err != nil {
		return fmt.Errorf("ensure artifact dir: %w", err)
	}
	deployments, err := op.buildDeployments()
	if err != nil {
		return err
	}

	var builder strings.Builder
	for idx, obj := range deployments {
		data, err := yaml.Marshal(obj)
		if err != nil {
			return fmt.Errorf("marshal kubernetes object: %w", err)
		}
		if idx > 0 {
			builder.WriteString("\n---\n")
		}
		builder.WriteString(string(data))
	}

	tmpFile := op.deploymentYml + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write deployment.yaml: %w", err)
	}

	if err := os.Rename(tmpFile, op.deploymentYml); err != nil {
		return fmt.Errorf("persist deployment.yaml: %w", err)
	}

	op.pushRollback(func(context.Context) error {
		return os.Remove(op.deploymentYml)
	})

	return nil
}

func (op *installOperation) buildDeployments() ([]interface{}, error) {
	manifest := op.manifestRes.Manifest

	frontendName := fmt.Sprintf("%s-frontend", op.identifier)
	backendName := fmt.Sprintf("%s-backend", op.identifier)

	frontendLabels := mergeLabels(manifest.Kubernetes.Labels, map[string]string{
		"app": frontendName,
	})
	backendLabels := mergeLabels(manifest.Kubernetes.Labels, map[string]string{
		"app": backendName,
	})

	frontendPort := resolvePort(manifest.Docker.Frontend.Ports, defaultServicePort)
	backendPort := resolvePort(manifest.Docker.Backend.Ports, backendServicePort)

	deployments := []interface{}{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      frontendName,
				Namespace: op.namespace,
				Labels:    frontendLabels,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: resolveReplicaPtr(manifest.Docker.Frontend.Replicas),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": frontendName,
					},
				},
				Template: apiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: frontendLabels,
					},
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:  frontendName,
								Image: buildImage(manifest.Docker.Frontend.Image, manifest.Docker.Frontend.Tag),
								Ports: []apiv1.ContainerPort{
									{ContainerPort: frontendPort},
								},
								Env: toEnvVars(manifest.Docker.Frontend.Env),
							},
						},
					},
				},
			},
		},
	}

	backendEnv := toEnvVars(manifest.Docker.Backend.Env)
	if op.dbCredentials != nil {
		backendEnv = append(backendEnv, apiv1.EnvVar{
			Name:  "DATABASE_URL",
			Value: op.dbCredentials.DSN,
		})
	}
	backendEnv = append(backendEnv, apiv1.EnvVar{
		Name:  "MIGRATION_ENABLED",
		Value: fmt.Sprintf("%v", manifest.Docker.Backend.Migration),
	})

	deployments = append(deployments,
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backendName,
				Namespace: op.namespace,
				Labels:    backendLabels,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: resolveReplicaPtr(manifest.Docker.Backend.Replicas),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": backendName,
					},
				},
				Template: apiv1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: backendLabels,
					},
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:  backendName,
								Image: buildImage(manifest.Docker.Backend.Image, manifest.Docker.Backend.Tag),
								Ports: []apiv1.ContainerPort{
									{ContainerPort: backendPort},
								},
								Env: backendEnv,
							},
						},
					},
				},
			},
		},
		&apiv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      frontendName,
				Namespace: op.namespace,
				Labels:    frontendLabels,
			},
			Spec: apiv1.ServiceSpec{
				Selector: map[string]string{
					"app": frontendName,
				},
				Ports: []apiv1.ServicePort{
					{
						Port:       frontendPort,
						TargetPort: intstr.FromInt(int(frontendPort)),
					},
				},
			},
		},
		&apiv1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      backendName,
				Namespace: op.namespace,
				Labels:    backendLabels,
			},
			Spec: apiv1.ServiceSpec{
				Selector: map[string]string{
					"app": backendName,
				},
				Ports: []apiv1.ServicePort{
					{
						Port:       backendPort,
						TargetPort: intstr.FromInt(int(backendPort)),
					},
				},
			},
		},
	)

	return deployments, nil
}

func (op *installOperation) applyDeployment(ctx context.Context) error {
	gr, err := restmapper.GetAPIGroupResources(op.installer.kube.Clientset.Discovery())
	if err != nil {
		return fmt.Errorf("discover api resources: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)

	if err := utils.ApplyManifest(op.deploymentYml, op.namespace, op.installer.kube.DynamicClient, mapper); err != nil {
		return fmt.Errorf("apply deployment manifest: %w", err)
	}

	op.pushRollback(func(context.Context) error {
		frontendName := fmt.Sprintf("%s-frontend", op.identifier)
		backendName := fmt.Sprintf("%s-backend", op.identifier)

		op.installer.kube.DeleteDeployment(op.namespace, frontendName)
		op.installer.kube.DeleteService(op.namespace, frontendName)
		op.installer.kube.DeleteDeployment(op.namespace, backendName)
		op.installer.kube.DeleteService(op.namespace, backendName)
		return nil
	})

	return nil
}

func (op *installOperation) ensureServicesAndIngress() error {
	frontendName := fmt.Sprintf("%s-frontend", op.identifier)
	backendName := fmt.Sprintf("%s-backend", op.identifier)

	frontendPath := fmt.Sprintf("/plugins/%s", op.req.RoutePath)
	backendPath := fmt.Sprintf("/plugins/%s/api", op.req.RoutePath)

	if op.req.RoutePath == "" {
		frontendPath = fmt.Sprintf("/plugins/%s", op.identifier)
		backendPath = fmt.Sprintf("/plugins/%s/api", op.identifier)
	}

	if err := op.installer.kube.AppendRuleToIngress(op.namespace, ingressName, frontendName, frontendPath); err != nil {
		return fmt.Errorf("append ingress rule frontend: %w", err)
	}
	if err := op.installer.kube.AppendRuleToIngress(op.namespace, ingressName, backendName, backendPath); err != nil {
		return fmt.Errorf("append ingress rule backend: %w", err)
	}

	op.frontendURL = fmt.Sprintf("%s.%s.svc.cluster.local", frontendName, op.namespace)
	op.backendURL = fmt.Sprintf("%s.%s.svc.cluster.local", backendName, op.namespace)

	op.pushRollback(func(context.Context) error {
		if err := op.installer.kube.DeleteRuleFromIngress(op.namespace, frontendPath, ingressName); err != nil {
			log.Errorf("rollback ingress frontend: %v", err)
		}
		if err := op.installer.kube.DeleteRuleFromIngress(op.namespace, backendPath, ingressName); err != nil {
			log.Errorf("rollback ingress backend: %v", err)
		}
		return nil
	})

	return nil
}

func (op *installOperation) handleLogo(ctx context.Context) error {
	newLogo, backup, err := processLogo(op.manifestRes.Manifest, op.extractDir, op.artifactDir)
	if err != nil {
		return err
	}
	op.logoPath = newLogo
	if backup != "" {
		op.pushRollback(func(context.Context) error {
			return restoreLogoBackup(newLogo, backup)
		})
	}
	return nil
}

func (op *installOperation) sendBackendSignals(ctx context.Context) error {
	if len(op.req.BackendCallbacks) == 0 {
		return nil
	}

	payload := map[string]interface{}{
		"engineKey":   op.manifestRes.Manifest.EngineKey,
		"releaseId":   op.req.ReleaseID,
		"version":     op.manifestRes.Manifest.Version,
		"frontendUrl": op.frontendURL,
		"backendUrl":  op.backendURL,
		"namespace":   op.namespace,
		"logoPath":    op.logoPath,
		"manifestSha": op.manifestRes.SHA256Hex,
		"permissions": op.manifestRes.Manifest.Permissions,
		"databaseDsn": "",
		"operation":   op.kind,
		"timestamp":   op.startTime.UTC(),
	}
	if op.dbCredentials != nil {
		payload["databaseDsn"] = op.dbCredentials.DSN
	}

	for key, url := range op.req.BackendCallbacks {
		if strings.TrimSpace(url) == "" {
			continue
		}
		if err := postJSON(ctx, op.installer.httpClient, url, payload); err != nil {
			return fmt.Errorf("callback %s: %w", key, err)
		}
	}
	return nil
}

func (op *installOperation) writeRegistryRecord() error {
	record := pluginRecord{
		Identifier:     op.identifier,
		EngineKey:      op.manifestRes.Manifest.EngineKey,
		ReleaseID:      op.req.ReleaseID,
		Version:        op.manifestRes.Manifest.Version,
		Namespace:      op.namespace,
		RoutePath:      op.req.RoutePath,
		ManifestSHA:    op.manifestRes.SHA256Hex,
		ManifestPath:   op.manifestRes.Path,
		DeploymentPath: op.deploymentYml,
		LogoPath:       op.logoPath,
		UpdatedAt:      time.Now().UTC(),
	}
	if op.dbCredentials != nil {
		record.Database = *op.dbCredentials
	}

	if err := savePluginRecord(record); err != nil {
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func safeCall(ctx context.Context, fn func(context.Context) error) (err error) {
	if fn == nil {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("panic during rollback: %v", r)
			if err == nil {
				err = fmt.Errorf("panic during rollback: %v", r)
			}
		}
	}()
	return fn(ctx)
}

func mergeLabels(base map[string]string, extra map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func resolvePort(ports []int32, fallback int32) int32 {
	if len(ports) == 0 {
		return fallback
	}
	if ports[0] == 0 {
		return fallback
	}
	return ports[0]
}

func resolveReplicaPtr(replica int32) *int32 {
	if replica <= 0 {
		replica = 1
	}
	return &replica
}

func buildImage(image, tag string) string {
	if tag == "" {
		return image
	}
	if strings.Contains(image, ":") {
		return image
	}
	return fmt.Sprintf("%s:%s", image, tag)
}

func toEnvVars(env map[string]string) []apiv1.EnvVar {
	if len(env) == 0 {
		return nil
	}
	var out []apiv1.EnvVar
	for k, v := range env {
		out = append(out, apiv1.EnvVar{Name: k, Value: v})
	}
	return out
}

func postJSON(ctx context.Context, client *http.Client, url string, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// -----------------------------------------------------------------------------
// registry persistence
// -----------------------------------------------------------------------------

type pluginRecord struct {
	Identifier     string              `json:"identifier"`
	EngineKey      string              `json:"engineKey"`
	ReleaseID      int64               `json:"releaseId"`
	Version        string              `json:"version"`
	Namespace      string              `json:"namespace"`
	RoutePath      string              `json:"routePath"`
	ManifestSHA    string              `json:"manifestSha"`
	ManifestPath   string              `json:"manifestPath"`
	DeploymentPath string              `json:"deploymentPath"`
	LogoPath       string              `json:"logoPath"`
	Database       databaseCredentials `json:"database"`
	UpdatedAt      time.Time           `json:"updatedAt"`
}

func recordPath(identifier string) string {
	file := fmt.Sprintf("%s.json", identifier)
	return filepath.Join(pluginsRegistryDir, file)
}

func savePluginRecord(record pluginRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plugin record: %w", err)
	}
	if err := os.WriteFile(recordPath(record.Identifier), data, 0o644); err != nil {
		return fmt.Errorf("write plugin record: %w", err)
	}
	return nil
}

func loadPluginRecord(identifier string) (*pluginRecord, error) {
	data, err := os.ReadFile(recordPath(identifier))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("plugin %s not found in registry", identifier)
		}
		return nil, fmt.Errorf("read plugin record: %w", err)
	}
	var record pluginRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parse plugin record: %w", err)
	}
	return &record, nil
}

func deletePluginRecord(identifier string) error {
	if err := os.Remove(recordPath(identifier)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func removeArtifacts(identifier string) error {
	return os.RemoveAll(filepath.Join(pluginsArtifactRoot, identifier))
}
