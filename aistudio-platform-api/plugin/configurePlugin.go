package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"Kubernetes-api/artifacts"
	"Kubernetes-api/helper"
	utils "Kubernetes-api/kubeutils"

	"github.com/gofiber/fiber/v2/log"
	apiv1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

var (
	kubeNameSanitizer = regexp.MustCompile(`[^a-z0-9-]`)
)

type pluginManifest struct {
	Name                 string            `json:"name" yaml:"name"`
	EngineKey            string            `json:"engine_key" yaml:"engine_key"`
	RoutePath            string            `json:"route_path" yaml:"route_path"`
	NodeSelector         string            `json:"node_selector" yaml:"node_selector"`
	PersistentVolumeSize string            `json:"persistent_volume_size" yaml:"persistent_volume_size"`
	Docker               manifestDocker    `json:"docker" yaml:"docker"`
	Env                  map[string]string `json:"env" yaml:"env"`
}

type manifestDocker struct {
	Frontend manifestComponent `json:"frontend" yaml:"frontend"`
	Backend  manifestBackend   `json:"backend" yaml:"backend"`
}

type manifestComponent struct {
	Image string `json:"image" yaml:"image"`
}

type manifestBackend struct {
	Image     string `json:"image" yaml:"image"`
	Migration bool   `json:"migration" yaml:"migration"`
}

func InstallPlugin(req PluginDeploymentsRequest) (string, error) {
	result, err := installPluginCore(req)
	if err != nil {
		return "", err
	}
	return formatInstallMessage(result), nil
}

func installPluginCore(req PluginDeploymentsRequest) (*PluginInstallResult, error) {
	engineKey := strings.TrimSpace(req.EngineKey)
	if engineKey == "" {
		engineKey = strings.TrimSpace(req.PluginName)
	}
	if engineKey == "" {
		return nil, fmt.Errorf("engineKey is required")
	}

	if req.ReleaseId == 0 {
		return nil, fmt.Errorf("releaseId is required")
	}

	downloadURL, err := buildPluginDownloadURL(req)
	if err != nil {
		return nil, err
	}

	sanitizedKey := sanitizeKubeName(engineKey)
	if sanitizedKey == "" {
		return nil, fmt.Errorf("failed to sanitize engine key %q", engineKey)
	}

	routePath := strings.TrimSpace(req.RoutePath)
	if routePath == "" {
		routePath = engineKey
	}
	sanitizedRoute := sanitizeRouteSegment(routePath)

	sourceArtifactsDir := filepath.Join("temp", "plugins")
	if err := os.MkdirAll(sourceArtifactsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create source artifacts directory: %w", err)
	}

	destinationDir := filepath.Join("artifact", fmt.Sprintf("%s-%d-extracted", sanitizedKey, req.ReleaseId))
	if err := os.RemoveAll(destinationDir); err != nil {
		return nil, fmt.Errorf("failed to clean destination directory: %w", err)
	}
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	zipFileName := fmt.Sprintf("%s-%d.zip", sanitizedKey, time.Now().UnixNano())
	zipFilePath := filepath.Join(sourceArtifactsDir, zipFileName)
	extractDir := filepath.Join(sourceArtifactsDir, fmt.Sprintf("%s-%d-extracted", sanitizedKey, req.ReleaseId))
	if err := os.RemoveAll(extractDir); err != nil {
		return nil, fmt.Errorf("failed to clean extraction directory: %w", err)
	}
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create extraction directory: %w", err)
	}

	if err := helper.DownloadFile(downloadURL, zipFilePath); err != nil {
		return nil, fmt.Errorf("failed to download plugin zip: %w", err)
	}
	defer os.Remove(zipFilePath)
	defer os.RemoveAll(extractDir)

	extractedFiles, err := helper.ExtractZip(zipFilePath, extractDir)
	if err != nil {
		return nil, fmt.Errorf("failed to extract zip: %w", err)
	}

	manifest, manifestPath, err := parseManifest(extractedFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	if manifest.EngineKey == "" {
		manifest.EngineKey = engineKey
	}

	if !strings.EqualFold(manifest.EngineKey, engineKey) {
		return nil, fmt.Errorf("manifest engine_key (%s) does not match request (%s)", manifest.EngineKey, engineKey)
	}

	if manifest.Name == "" {
		manifest.Name = engineKey
	}

	if manifest.RoutePath != "" {
		sanitizedRoute = sanitizeRouteSegment(manifest.RoutePath)
	}

	if manifest.Docker.Frontend.Image == "" {
		return nil, errors.New("manifest missing docker.frontend.image")
	}
	if manifest.Docker.Backend.Image == "" {
		return nil, errors.New("manifest missing docker.backend.image")
	}

	volumeSize := strings.TrimSpace(manifest.PersistentVolumeSize)
	if volumeSize == "" {
		volumeSize = "10Gi"
	}

	nodeSelector := strings.TrimSpace(req.NodeSelector)
	if nodeSelector == "" {
		nodeSelector = strings.TrimSpace(manifest.NodeSelector)
	}
	if nodeSelector == "" {
		nodeSelector = NodeSelector
	}

	kubeConfig := utils.NewKubernetesConfig()
	if kubeConfig == nil || kubeConfig.Clientset == nil {
		return nil, errors.New("failed to initialize kubernetes configuration")
	}

	if err := ensureNamespaceExists(context.Background(), kubeConfig, pluginNamespace); err != nil {
		return nil, err
	}

	pvcName := fmt.Sprintf("%s-data", sanitizedKey)
	if err := kubeConfig.CreatePersistentVolume(pluginNamespace, pvcName, volumeSize); err != nil {
		if !isAlreadyExistsError(err) {
			return nil, fmt.Errorf("failed to create PVC: %w", err)
		}
	}

	copyMsg, err := artifacts.CopyAllArtifacts(extractDir, destinationDir)
	if err != nil {
		return nil, fmt.Errorf("failed to copy plugin artifacts: %w", err)
	}
	log.Infof("plugin artifacts copied: %s", copyMsg)

	envVars := buildEnvVars(manifest)

	frontendDeploymentName := fmt.Sprintf("%s-frontend", sanitizedKey)
	backendDeploymentName := fmt.Sprintf("%s-backend", sanitizedKey)
	frontendServiceName := frontendDeploymentName
	backendServiceName := backendDeploymentName

	if !kubeConfig.ModelDeploymentExists(pluginNamespace, frontendDeploymentName) {
		if _, err := kubeConfig.ConfigDeployment(
			pluginNamespace,
			frontendDeploymentName,
			manifest.Docker.Frontend.Image,
			80,
			nodeSelector,
			envVars,
		); err != nil {
			return nil, fmt.Errorf("failed to create frontend deployment: %w", err)
		}
	}

	if !kubeConfig.ModelDeploymentExists(pluginNamespace, backendDeploymentName) {
		backendEnv := append([]apiv1.EnvVar{}, envVars...)
		backendEnv = append(backendEnv, apiv1.EnvVar{
			Name:  "MIGRATION_ENABLED",
			Value: strconv.FormatBool(manifest.Docker.Backend.Migration),
		})

		if _, err := kubeConfig.ConfigDeployment(
			pluginNamespace,
			backendDeploymentName,
			manifest.Docker.Backend.Image,
			8080,
			nodeSelector,
			backendEnv,
		); err != nil {
			return nil, fmt.Errorf("failed to create backend deployment: %w", err)
		}
	}

	if !kubeConfig.ServiceExists(pluginNamespace, frontendServiceName) {
		kubeConfig.CreateService(pluginNamespace, frontendServiceName, frontendDeploymentName, 80, apiv1.ServiceTypeClusterIP)
	}
	if !kubeConfig.ServiceExists(pluginNamespace, backendServiceName) {
		kubeConfig.CreateService(pluginNamespace, backendServiceName, backendDeploymentName, 8080, apiv1.ServiceTypeClusterIP)
	}

	frontendPath := fmt.Sprintf("/plugins/%s", sanitizedRoute)
	backendPath := fmt.Sprintf("/plugins/%s/api", sanitizedRoute)

	if err := kubeConfig.AppendRuleToIngress(pluginNamespace, ingressName, frontendServiceName, frontendPath); err != nil {
		return nil, fmt.Errorf("failed to append frontend ingress rule: %w", err)
	}
	if err := kubeConfig.AppendRuleToIngress(pluginNamespace, ingressName, backendServiceName, backendPath); err != nil {
		return nil, fmt.Errorf("failed to append backend ingress rule: %w", err)
	}

	result := &PluginInstallResult{
		FrontendURL:       frontendPath,
		BackendURL:        backendPath,
		Namespace:         pluginNamespace,
		ManifestPath:      manifestPath,
		ArtifactsLocation: destinationDir,
	}

	return result, nil
}

func formatInstallMessage(result *PluginInstallResult) string {
	return fmt.Sprintf(
		"Plugin installed successfully!\nFrontend: %s\nBackend: %s\nNamespace: %s",
		result.FrontendURL,
		result.BackendURL,
		result.Namespace,
	)
}

func buildPluginDownloadURL(req PluginDeploymentsRequest) (string, error) {
	if url := strings.TrimSpace(req.DownloadURL); url != "" {
		return url, nil
	}
	if url := strings.TrimSpace(req.ZipURL); url != "" {
		return url, nil
	}

	base := strings.TrimSpace(ZipURL)
	if base == "" {
		return "", errors.New("downloadUrl (zipUrl) is required")
	}

	endpoint := strings.Trim(strings.TrimSpace(ZipEndpoint), "/")
	base = strings.TrimRight(base, "/")

	full := base
	if endpoint != "" {
		full = fmt.Sprintf("%s/%s", base, endpoint)
	}

	return fmt.Sprintf("%s/%d", full, req.ReleaseId), nil
}

func parseManifest(extractedFiles []string) (*pluginManifest, string, error) {
	for _, file := range extractedFiles {
		base := strings.ToLower(filepath.Base(file))
		if base == "manifest.json" || base == "manifest.yaml" || base == "manifest.yml" {
			manifest, err := loadManifest(file)
			return manifest, file, err
		}
	}
	return nil, "", errors.New("manifest file not found in plugin archive")
}

func loadManifest(path string) (*pluginManifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest pluginManifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	return &manifest, nil
}

func sanitizeKubeName(value string) string {
	if value == "" {
		return ""
	}
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, ".", "-")
	value = strings.ReplaceAll(value, " ", "-")
	value = kubeNameSanitizer.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		value = "plugin"
	}
	return value
}

func sanitizeRouteSegment(value string) string {
	value = strings.Trim(value, "/")
	if value == "" {
		return "plugin"
	}
	return sanitizeKubeName(value)
}

func ensureNamespaceExists(ctx context.Context, kubeConfig *utils.KubernetesConfig, namespace string) error {
	if kubeConfig == nil || kubeConfig.Clientset == nil {
		return errors.New("kubernetes configuration is not initialized")
	}

	nsClient := kubeConfig.Clientset.CoreV1().Namespaces()
	_, err := nsClient.Get(ctx, namespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to check namespace %s: %w", namespace, err)
	}

	_, err = nsClient.Create(ctx, &apiv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create namespace %s: %w", namespace, err)
	}
	return nil
}

func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}

func buildEnvVars(manifest *pluginManifest) []apiv1.EnvVar {
	envVars := []apiv1.EnvVar{
		{Name: "PLUGIN_NAME", Value: manifest.Name},
		{Name: "PLUGIN_ENGINE_KEY", Value: manifest.EngineKey},
	}

	seen := map[string]struct{}{
		"PLUGIN_NAME":       {},
		"PLUGIN_ENGINE_KEY": {},
	}

	for key, value := range manifest.Env {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		envVars = append(envVars, apiv1.EnvVar{Name: key, Value: value})
		seen[key] = struct{}{}
	}

	return envVars
}

func DeletePluginDeployments(pluginName string, rulePath string) error {
	key := strings.TrimSpace(pluginName)
	if key == "" {
		key = strings.TrimSpace(rulePath)
	}
	if key == "" {
		return errors.New("pluginName or rulePath is required")
	}

	sanitizedKey := sanitizeKubeName(key)
	kubeConfig := utils.NewKubernetesConfig()
	if kubeConfig == nil || kubeConfig.Clientset == nil {
		return errors.New("failed to initialize kubernetes configuration")
	}

	frontendDeploymentName := fmt.Sprintf("%s-frontend", sanitizedKey)
	backendDeploymentName := fmt.Sprintf("%s-backend", sanitizedKey)

	kubeConfig.DeleteDeployment(pluginNamespace, frontendDeploymentName)
	kubeConfig.DeleteService(pluginNamespace, frontendDeploymentName)

	sanitizedRoute := sanitizeRouteSegment(rulePath)
	frontendPath := fmt.Sprintf("/plugins/%s", sanitizedRoute)
	backendPath := fmt.Sprintf("/plugins/%s/api", sanitizedRoute)

	if err := kubeConfig.DeleteRuleFromIngress(pluginNamespace, frontendPath, ingressName); err != nil {
		log.Warnf("failed to delete frontend ingress rule %s: %v", frontendPath, err)
	}
	if err := kubeConfig.DeleteRuleFromIngress(pluginNamespace, backendPath, ingressName); err != nil {
		log.Warnf("failed to delete backend ingress rule %s: %v", backendPath, err)
	}

	kubeConfig.DeleteDeployment(pluginNamespace, backendDeploymentName)
	kubeConfig.DeleteService(pluginNamespace, backendDeploymentName)

	return nil
}

func ListPlugins() ([]map[string]string, error) {
	kubeConfig := utils.NewKubernetesConfig()
	if kubeConfig == nil {
		return nil, errors.New("failed to initialize kubernetes configuration")
	}
	return kubeConfig.ListPods(pluginNamespace)
}
