package plugin

import (
	"Kubernetes-api/artifacts"
	"Kubernetes-api/helper"
	utils "Kubernetes-api/kubeutils"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type PluginManifest struct {
	Name         string             `json:"name" yaml:"name"`
	EngineKey    string             `json:"engine_key" yaml:"engine_key"`
	RoutePath    string             `json:"route_path" yaml:"route_path"`
	NodeSelector string             `json:"node_selector" yaml:"node_selector"`
	Env          map[string]string  `json:"env" yaml:"env"`
	Docker       PluginManifestSpec `json:"docker" yaml:"docker"`
	Storage      PluginStorageSpec  `json:"storage" yaml:"storage"`
}

type PluginManifestSpec struct {
	Frontend PluginContainerSpec `json:"frontend" yaml:"frontend"`
	Backend  PluginBackendSpec   `json:"backend" yaml:"backend"`
}

type PluginContainerSpec struct {
	Image string `json:"image" yaml:"image"`
	Port  int    `json:"port" yaml:"port"`
}

type PluginBackendSpec struct {
	Image     string `json:"image" yaml:"image"`
	Port      int    `json:"port" yaml:"port"`
	Migration bool   `json:"migration" yaml:"migration"`
}

type PluginStorageSpec struct {
	PVCSize string `json:"pvc_size" yaml:"pvc_size"`
}

func InstallPlugin(req PluginDeploymentsRequest) (string, error) {
	engineKey := strings.TrimSpace(req.EngineKey)
	if engineKey == "" {
		return "", fmt.Errorf("engineKey is required")
	}
	if req.ReleaseId == 0 {
		return "", fmt.Errorf("releaseId is required")
	}

	sourceArtifactsDir := filepath.Join(os.TempDir(), "plugins")
	destinationDir := filepath.Join("artifact", fmt.Sprintf("%s-%d-extracted", engineKey, req.ReleaseId))
	zipFileName := fmt.Sprintf("%s-%d.zip", engineKey, time.Now().UnixNano())
	zipFilePath := filepath.Join(sourceArtifactsDir, zipFileName)
	extractDir := filepath.Join(sourceArtifactsDir, fmt.Sprintf("%s-%d-extracted", engineKey, req.ReleaseId))

	if err := os.MkdirAll(sourceArtifactsDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create artifacts dir: %w", err)
	}
	if err := os.MkdirAll(destinationDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create destination dir: %w", err)
	}

	if err := os.RemoveAll(extractDir); err != nil {
		return "", fmt.Errorf("failed to reset extract dir: %w", err)
	}
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create extract dir: %w", err)
	}

	downloadURL, err := buildReleaseDownloadURL(req)
	if err != nil {
		return "", err
	}

	if err := helper.DownloadFile(downloadURL, zipFilePath); err != nil {
		return "", fmt.Errorf("failed to download plugin zip: %w", err)
	}
	defer os.Remove(zipFilePath)

	extractedFiles, err := helper.ExtractZip(zipFilePath, extractDir)
	if err != nil {
		return "", fmt.Errorf("failed to extract zip: %w", err)
	}
	defer os.RemoveAll(extractDir)

	manifest, manifestPath, err := parseManifest(extractedFiles)
	if err != nil {
		return "", fmt.Errorf("failed to parse manifest: %w", err)
	}
	manifest.applyDefaults()

	if manifest.EngineKey != engineKey {
		return "", fmt.Errorf("manifest engine_key (%s) does not match request (%s)", manifest.EngineKey, engineKey)
	}

	if manifest.Docker.Frontend.Image == "" {
		return "", fmt.Errorf("frontend image missing in manifest: %s", manifestPath)
	}
	if manifest.Docker.Backend.Image == "" {
		return "", fmt.Errorf("backend image missing in manifest: %s", manifestPath)
	}

	if kc == nil {
		kc = utils.NewKubernetesConfig()
	}
	if kc == nil {
		return "", fmt.Errorf("failed to initialize kubernetes client")
	}

	kc.CreateNamespace(pluginNamespace)

	pvcName := fmt.Sprintf("%s-data", engineKey)
	diskStorage := manifest.Storage.PVCSize
	if diskStorage == "" {
		diskStorage = "10Gi"
	}

	if err := kc.CreatePersistentVolume(pluginNamespace, pvcName, diskStorage); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return "", fmt.Errorf("failed to create PVC: %w", err)
		}
	}

	ignoreFilePath := filepath.Join(extractDir, ".studioignore")
	if _, err := os.Stat(ignoreFilePath); os.IsNotExist(err) {
		if err := os.WriteFile(ignoreFilePath, []byte{}, 0o644); err != nil {
			return "", fmt.Errorf("failed to create .studioignore: %w", err)
		}
	}

	if _, err := artifacts.CopyAllArtifacts(extractDir, destinationDir); err != nil {
		return "", fmt.Errorf("failed to copy plugin artifacts: %w", err)
	}

	envVars := buildEnvVars(manifest.Env)
	envVars = append(envVars, apiv1.EnvVar{Name: "PLUGIN_NAME", Value: manifest.Name})

	nodeSelector := manifest.NodeSelector
	if nodeSelector == "" {
		nodeSelector = defaultNodeSelector
	}

	frontendDeploymentName := fmt.Sprintf("%s-frontend", engineKey)
	backendDeploymentName := fmt.Sprintf("%s-backend", engineKey)
	frontendServiceName := frontendDeploymentName
	backendServiceName := backendDeploymentName

	if !kc.ModelDeploymentExists(pluginNamespace, frontendDeploymentName) {
		if _, err := kc.ConfigDeployment(
			pluginNamespace,
			frontendDeploymentName,
			manifest.Docker.Frontend.Image,
			manifest.Docker.Frontend.Port,
			nodeSelector,
			envVars,
		); err != nil {
			return "", fmt.Errorf("failed to create frontend deployment: %w", err)
		}
	}

	backendEnv := append([]apiv1.EnvVar{}, envVars...)
	backendEnv = append(backendEnv, apiv1.EnvVar{
		Name:  "MIGRATION_ENABLED",
		Value: strconv.FormatBool(manifest.Docker.Backend.Migration),
	})

	if !kc.ModelDeploymentExists(pluginNamespace, backendDeploymentName) {
		if _, err := kc.ConfigDeployment(
			pluginNamespace,
			backendDeploymentName,
			manifest.Docker.Backend.Image,
			manifest.Docker.Backend.Port,
			nodeSelector,
			backendEnv,
		); err != nil {
			return "", fmt.Errorf("failed to create backend deployment: %w", err)
		}
	}

	if !kc.ServiceExists(pluginNamespace, frontendServiceName) {
		kc.CreateService(pluginNamespace, frontendServiceName, frontendDeploymentName, manifest.Docker.Frontend.Port, apiv1.ServiceTypeClusterIP)
	}
	if !kc.ServiceExists(pluginNamespace, backendServiceName) {
		kc.CreateService(pluginNamespace, backendServiceName, backendDeploymentName, manifest.Docker.Backend.Port, apiv1.ServiceTypeClusterIP)
	}

	routePath := strings.TrimSpace(req.RoutePath)
	if routePath == "" {
		routePath = strings.TrimSpace(manifest.RoutePath)
	}
	if routePath == "" {
		routePath = engineKey
	}

	frontendPath := fmt.Sprintf("/plugins/%s", routePath)
	backendPath := fmt.Sprintf("/plugins/%s/api", routePath)

	if err := kc.AppendRuleToIngress(pluginNamespace, ingressName, frontendServiceName, frontendPath); err != nil {
		return "", fmt.Errorf("failed to append frontend ingress rule: %w", err)
	}
	if err := kc.AppendRuleToIngress(pluginNamespace, ingressName, backendServiceName, backendPath); err != nil {
		return "", fmt.Errorf("failed to append backend ingress rule: %w", err)
	}

	return fmt.Sprintf("Frontend URL: %s, Backend URL: %s", frontendPath, backendPath), nil
}

func parseManifest(files []string) (*PluginManifest, string, error) {
	for _, filePath := range files {
		if !isManifestCandidate(filePath) {
			continue
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read manifest file %s: %w", filePath, err)
		}

		manifest := &PluginManifest{}
		switch strings.ToLower(filepath.Ext(filePath)) {
		case ".yaml", ".yml":
			if err := yaml.Unmarshal(data, manifest); err != nil {
				return nil, "", fmt.Errorf("failed to decode manifest yaml %s: %w", filePath, err)
			}
		case ".json":
			if err := json.Unmarshal(data, manifest); err != nil {
				return nil, "", fmt.Errorf("failed to decode manifest json %s: %w", filePath, err)
			}
		default:
			continue
		}

		if strings.TrimSpace(manifest.EngineKey) == "" {
			continue
		}

		return manifest, filePath, nil
	}

	return nil, "", fmt.Errorf("manifest file not found in plugin archive")
}

func isManifestCandidate(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".json") {
		if strings.Contains(base, "manifest") || strings.Contains(base, "plugin") {
			return true
		}
	}
	return false
}

func buildEnvVars(env map[string]string) []apiv1.EnvVar {
	if len(env) == 0 {
		return []apiv1.EnvVar{}
	}

	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var envVars []apiv1.EnvVar
	for _, key := range keys {
		envVars = append(envVars, apiv1.EnvVar{
			Name:  key,
			Value: env[key],
		})
	}

	return envVars
}

func (m *PluginManifest) applyDefaults() {
	if strings.TrimSpace(m.EngineKey) == "" {
		return
	}
	if strings.TrimSpace(m.Name) == "" {
		m.Name = m.EngineKey
	}
	if strings.TrimSpace(m.Storage.PVCSize) == "" {
		m.Storage.PVCSize = "10Gi"
	}
	if m.Docker.Frontend.Port == 0 {
		m.Docker.Frontend.Port = 80
	}
	if m.Docker.Backend.Port == 0 {
		m.Docker.Backend.Port = 8080
	}
	if m.Env == nil {
		m.Env = map[string]string{}
	}
	if strings.TrimSpace(m.RoutePath) == "" {
		m.RoutePath = m.EngineKey
	}
}

func buildReleaseDownloadURL(req PluginDeploymentsRequest) (string, error) {
	if strings.TrimSpace(req.ZipURL) != "" {
		return req.ZipURL, nil
	}

	base := strings.TrimSpace(zipBaseURL)
	if base == "" {
		return "", fmt.Errorf("zip base URL is not configured; set PLUGIN_ZIP_BASE_URL or provide zipUrl")
	}

	endpoint := strings.TrimSpace(zipEndpoint)
	full := strings.TrimRight(base, "/") + "/" + strings.TrimLeft(endpoint, "/")
	parsed, err := url.Parse(full)
	if err != nil {
		return "", fmt.Errorf("invalid zip download URL %q: %w", full, err)
	}

	query := parsed.Query()
	if req.ReleaseId > 0 {
		query.Set("releaseId", strconv.Itoa(req.ReleaseId))
	}
	if strings.TrimSpace(req.EngineKey) != "" {
		query.Set("engineKey", strings.TrimSpace(req.EngineKey))
	}
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}
