package plugin

import (
	"Kubernetes-api/artifacts"
	"Kubernetes-api/helper"
	utils "Kubernetes-api/kubeutils"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/restmapper"
)

func CreatePluginDeployments(req PluginDeploymentsRequest) (string, error) {
	if req.ZipURL == "" {
		return "", fmt.Errorf("zipUrl is required")
	}
	if req.PluginName == "" {
		return "", fmt.Errorf("pluginName is required")
	}
	if req.RoutePath == "" {
		return "", fmt.Errorf("routePath is required")
	}


	artifactsDir := "artifacts/plugins"
	zipFileName := fmt.Sprintf("%s-%d.zip", req.PluginName, time.Now().UnixNano())
	zipFilePath := filepath.Join(artifactsDir, zipFileName)
	extractDir := filepath.Join(artifactsDir, req.PluginName+"-extracted")

	kc.CreateNamespace(pluginNamespace)

	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artifacts dir: %w", err)
	}

	if err := helper.DownloadFile(req.ZipURL, zipFilePath); err != nil {
		return "", fmt.Errorf("failed to download zip from %s: %w", req.ZipURL, err)
	}
	defer os.Remove(zipFilePath)

	if err := ApplyManifestsFromZip(zipFilePath, extractDir, pluginNamespace); err != nil {
		return "", fmt.Errorf("failed to apply manifests: %w", err)
	}

	// Step 5: Define expected service names
	frontendServiceName := fmt.Sprintf("%s-frontend", req.PluginName)
	backendServiceName := fmt.Sprintf("%s-backend", req.PluginName)

	port := 80
	if !kc.ServiceExists(pluginNamespace, frontendServiceName) {
		kc.CreateService(pluginNamespace, frontendServiceName, frontendServiceName, port, apiv1.ServiceTypeClusterIP)
	}
	if !kc.ServiceExists(pluginNamespace, backendServiceName) {
		kc.CreateService(pluginNamespace, backendServiceName, backendServiceName, port, apiv1.ServiceTypeClusterIP)
	}

	frontendPath := fmt.Sprintf("/plugins/%s", req.RoutePath)
	backendPath := fmt.Sprintf("/plugins/%s/api", req.RoutePath)

	kc.AppendRuleToIngress(pluginNamespace, ingressName, frontendServiceName, frontendPath)
	kc.AppendRuleToIngress(pluginNamespace, ingressName, backendServiceName, backendPath)

	frontendURL := fmt.Sprintf("http://%s.%s", frontendServiceName, pluginNamespace)
	backendURL := fmt.Sprintf("http://%s.%s", backendServiceName, pluginNamespace)

	message := fmt.Sprintf("Frontend URL: %s, Backend URL: %s", frontendURL, backendURL)
	return message, nil
}

func DeletePluginDeployments(pluginName string, rulePath string) error {
	frontendDeploymentName := strings.Replace(fmt.Sprintf("%s-frontend", pluginName), ".", "-", -1)
	frontendServiceName := frontendDeploymentName

	backendDeploymentName := strings.Replace(fmt.Sprintf("%s-backend", pluginName), ".", "-", -1)
	backendServiceName := backendDeploymentName

	kc.DeleteDeployment(pluginNamespace, frontendDeploymentName)
	kc.DeleteService(pluginNamespace, frontendServiceName)
	kc.DeleteRuleFromIngress(pluginNamespace, rulePath, ingressName)

	kc.DeleteDeployment(pluginNamespace, backendDeploymentName)
	kc.DeleteService(pluginNamespace, backendServiceName)

	return nil
}

func ApplyManifestsFromZip(zipFilePath, extractDir, namespace string) error {
	kube := utils.NewKubernetesConfig()
	
	files, err := helper.ExtractZip(zipFilePath, extractDir)
	if err != nil {
		return fmt.Errorf("failed to extract zip: %w", err)
	}

	gr, err := restmapper.GetAPIGroupResources(kube.Clientset.Discovery())
	if err != nil {
		return fmt.Errorf("failed to get API resources: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)

	for _, filePath := range files {
		if err := utils.ApplyManifest(filePath, namespace, kube.DynamicClient, mapper); err != nil {
			return err
		}
	}

	return nil
}

func ListPlugins() ([]map[string]string, error) {
	return kc.ListPods(pluginNamespace)
}

// PluginManifest represents the structure of a plugin manifest file
type PluginManifest struct {
	EngineKey string `json:"engine_key"`
	Name      string `json:"name"`
	Docker    struct {
		Frontend struct {
			Image string `json:"image"`
		} `json:"frontend"`
		Backend struct {
			Image     string `json:"image"`
			Migration bool   `json:"migration"`
		} `json:"backend"`
	} `json:"docker"`
	Database struct {
		Enabled bool   `json:"enabled"`
		Type    string `json:"type"` // e.g., "postgres", "mysql"
		Image   string `json:"image"`
	} `json:"database,omitempty"`
}

// parseManifest parses the manifest.json file from extracted files
func parseManifest(extractedFiles []string) (*PluginManifest, string, error) {
	var manifestPath string
	for _, file := range extractedFiles {
		if strings.HasSuffix(file, "manifest.json") || strings.HasSuffix(file, "plugin.json") {
			manifestPath = file
			break
		}
	}

	if manifestPath == "" {
		return nil, "", fmt.Errorf("manifest file not found in extracted files")
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, "", fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	return &manifest, manifestPath, nil
}

// InstallPlugin installs a plugin from a ZIP file downloaded from an API
func InstallPlugin(req PluginDeploymentsRequest) (string, error) {
	if req.EngineKey == "" {
		return "", fmt.Errorf("engineKey is required")
	}
	if req.ReleaseId == 0 {
		return "", fmt.Errorf("releaseId is required")
	}

	// === 1. Setup paths ===
	sourceArtifactsDir := "temp/plugins"
	destinationDir := fmt.Sprintf("%s/%s-%d-extracted", "artifact", req.EngineKey, req.ReleaseId)
	zipFileName := fmt.Sprintf("%s-%d.zip", req.EngineKey, time.Now().UnixNano())
	zipFilePath := filepath.Join(sourceArtifactsDir, zipFileName)
	extractDir := filepath.Join(sourceArtifactsDir, fmt.Sprintf("%s-%d-extracted", req.EngineKey, req.ReleaseId))

	// === 2. Create directories ===
	if err := os.MkdirAll(sourceArtifactsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create artifacts dir: %w", err)
	}
	if err := os.MkdirAll(destinationDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination dir: %w", err)
	}

	// === 3. Download ZIP from API ===
	apiURL := fmt.Sprintf("%s%s?releaseId=%d", ZipURL, ZipEndpoint, req.ReleaseId)
	if err := helper.DownloadFile(apiURL, zipFilePath); err != nil {
		return "", fmt.Errorf("failed to download plugin zip: %w", err)
	}
	defer os.Remove(zipFilePath)

	// === 4. Extract ZIP ===
	extractedFiles, err := helper.ExtractZip(zipFilePath, extractDir)
	if err != nil {
		return "", fmt.Errorf("failed to extract zip: %w", err)
	}

	// === 5. Parse manifest ===
	manifest, _, err := parseManifest(extractedFiles)
	if err != nil {
		return "", fmt.Errorf("failed to parse manifest: %w", err)
	}

	// === 6. Validate EngineKey ===
	if manifest.EngineKey != req.EngineKey {
		return "", fmt.Errorf("manifest engine_key (%s) does not match request (%s)", manifest.EngineKey, req.EngineKey)
	}

	// === 7. Kubernetes Setup ===
	kc := utils.NewKubernetesConfig()

	// Create namespace
	kc.CreateNamespace(pluginNamespace)

	// === 8. Define service & deployment names ===
	frontendDeploymentName := fmt.Sprintf("%s-frontend", req.EngineKey)
	backendDeploymentName := fmt.Sprintf("%s-backend", req.EngineKey)
	frontendServiceName := frontendDeploymentName
	backendServiceName := backendDeploymentName

	// === 9. Create PVCs (if needed) ===
	// TODO: Make PVC size configurable per plugin or from manifest
	diskStorage := "10Gi" // Default
	pvcName := fmt.Sprintf("%s-data", req.EngineKey)

	if err := kc.CreatePersistentVolume(pluginNamespace, pvcName, diskStorage); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return "", fmt.Errorf("failed to create PVC: %w", err)
		}
	}

	// === 10. Copy artifacts ===
	result, err := artifacts.CopyAllArtifacts(extractDir, destinationDir)
	if err != nil {
		return "", fmt.Errorf("error copying artifacts: %w", err)
	}
	fmt.Println("Files copied from source to destination for backend:", result)

	// === 11. Create database with access for the first time and run migration as init container ===
	// If error occurs then exit with revert migration
	if manifest.Database.Enabled && manifest.Docker.Backend.Migration {
		dbName := fmt.Sprintf("%s-db", req.EngineKey)
		dbDeploymentName := fmt.Sprintf("%s-database", req.EngineKey)
		dbServiceName := dbDeploymentName

		// Create database deployment if it doesn't exist
		if !kc.DeploymentExists(pluginNamespace, dbDeploymentName) {
			dbImage := manifest.Database.Image
			if dbImage == "" {
				// Default to postgres if not specified
				if manifest.Database.Type == "mysql" {
					dbImage = "mysql:8.0"
				} else {
					dbImage = "postgres:15"
				}
			}

			dbEnvVars := []apiv1.EnvVar{
				{Name: "POSTGRES_DB", Value: dbName},
				{Name: "POSTGRES_USER", Value: "plugin_user"},
				{Name: "POSTGRES_PASSWORD", Value: "plugin_password"},
			}
			if manifest.Database.Type == "mysql" {
				dbEnvVars = []apiv1.EnvVar{
					{Name: "MYSQL_DATABASE", Value: dbName},
					{Name: "MYSQL_USER", Value: "plugin_user"},
					{Name: "MYSQL_PASSWORD", Value: "plugin_password"},
					{Name: "MYSQL_ROOT_PASSWORD", Value: "root_password"},
				}
			}

			_, err := kc.ConfigDeployment(
				pluginNamespace,
				dbDeploymentName,
				dbImage,
				5432, // Default postgres port
				NodeSelector,
				dbEnvVars,
			)
			if err != nil {
				return "", fmt.Errorf("failed to create database deployment: %w", err)
			}

			// Create database service
			if !kc.ServiceExists(pluginNamespace, dbServiceName) {
				port := 5432
				if manifest.Database.Type == "mysql" {
					port = 3306
				}
				kc.CreateService(pluginNamespace, dbServiceName, dbDeploymentName, port, apiv1.ServiceTypeClusterIP)
			}
		}
	}

	// === 12. Prepare environment variables ===
	envVars := []apiv1.EnvVar{
		{Name: "PLUGIN_NAME", Value: manifest.Name},
	}

	// Add database connection env vars if database is enabled
	if manifest.Database.Enabled {
		dbServiceName := fmt.Sprintf("%s-database", req.EngineKey)
		dbName := fmt.Sprintf("%s-db", req.EngineKey)
		envVars = append(envVars,
			apiv1.EnvVar{Name: "DB_HOST", Value: dbServiceName},
			apiv1.EnvVar{Name: "DB_NAME", Value: dbName},
			apiv1.EnvVar{Name: "DB_USER", Value: "plugin_user"},
			apiv1.EnvVar{Name: "DB_PASSWORD", Value: "plugin_password"},
		)
	}

	// === 13. Node selector (optional) ===
	nodeSelector := NodeSelector // TODO: Make configurable or from manifest

	// === 14. Create Frontend Deployment ===
	if !kc.DeploymentExists(pluginNamespace, frontendDeploymentName) {
		_, err := kc.ConfigDeployment(
			pluginNamespace,
			frontendDeploymentName,
			manifest.Docker.Frontend.Image,
			80, // port
			nodeSelector,
			envVars,
		)
		if err != nil {
			return "", fmt.Errorf("failed to create frontend deployment: %w", err)
		}
	}

	// === 15. Create Backend Deployment with migration init container ===
	if !kc.DeploymentExists(pluginNamespace, backendDeploymentName) {
		backendEnv := append(envVars, apiv1.EnvVar{
			Name: "MIGRATION_ENABLED", Value: strconv.FormatBool(manifest.Docker.Backend.Migration),
		})

		// Create backend deployment
		deploymentName, err := kc.ConfigDeployment(
			pluginNamespace,
			backendDeploymentName,
			manifest.Docker.Backend.Image,
			8080, // backend port
			nodeSelector,
			backendEnv,
		)
		if err != nil {
			return "", fmt.Errorf("failed to create backend deployment: %w", err)
		}

		// Add init container for migration if migration is enabled
		if manifest.Docker.Backend.Migration {
			err := addMigrationInitContainer(kc, pluginNamespace, backendDeploymentName, manifest)
			if err != nil {
				// Revert: Delete the deployment if migration setup fails
				kc.DeleteDeployment(pluginNamespace, backendDeploymentName)
				return "", fmt.Errorf("failed to setup migration init container, deployment reverted: %w", err)
			}
			_ = deploymentName // Use deploymentName to avoid unused variable warning
		}
	}

	// === 16. Create Services ===
	port := 80
	if !kc.ServiceExists(pluginNamespace, frontendServiceName) {
		kc.CreateService(pluginNamespace, frontendServiceName, frontendDeploymentName, port, apiv1.ServiceTypeClusterIP)
	}
	if !kc.ServiceExists(pluginNamespace, backendServiceName) {
		kc.CreateService(pluginNamespace, backendServiceName, backendDeploymentName, 8080, apiv1.ServiceTypeClusterIP)
	}

	// === 17. Update Ingress ===
	frontendPath := fmt.Sprintf("/plugins/%s", req.EngineKey)
	backendPath := fmt.Sprintf("/plugins/%s/api", req.EngineKey)

	if err := kc.AppendRuleToIngress(pluginNamespace, ingressName, frontendServiceName, frontendPath); err != nil {
		return "", fmt.Errorf("failed to append frontend ingress rule: %w", err)
	}
	if err := kc.AppendRuleToIngress(pluginNamespace, ingressName, backendServiceName, backendPath); err != nil {
		return "", fmt.Errorf("failed to append backend ingress rule: %w", err)
	}

	// === 18. Construct URLs ===
	frontendURL := frontendPath // TODO: Use real domain
	backendURL := backendPath

	// === 19. Return success message ===
	message := fmt.Sprintf(
		"Plugin installed successfully!\nFrontend: %s\nBackend: %s\nNamespace: %s",
		frontendURL, backendURL, pluginNamespace,
	)
	return message, nil
}

// addMigrationInitContainer adds an init container to the backend deployment for running migrations
func addMigrationInitContainer(kc *utils.KubernetesConfig, namespace, deploymentName string, manifest *PluginManifest) error {
	deploymentsClient := kc.Clientset.AppsV1().Deployments(namespace)

	deployment, err := deploymentsClient.Get(context.Background(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Create init container for migration
	migrationInitContainer := apiv1.Container{
		Name:  "migration",
		Image: manifest.Docker.Backend.Image,
		Command: []string{"sh", "-c"},
		Args:   []string{"echo 'Running migrations...' && sleep 5"}, // TODO: Replace with actual migration command
		Env: []apiv1.EnvVar{
			{Name: "MIGRATION_ENABLED", Value: "true"},
		},
	}

	// Add database connection env vars if database is enabled
	if manifest.Database.Enabled {
		dbServiceName := fmt.Sprintf("%s-database", manifest.EngineKey)
		dbName := fmt.Sprintf("%s-db", manifest.EngineKey)
		migrationInitContainer.Env = append(migrationInitContainer.Env,
			apiv1.EnvVar{Name: "DB_HOST", Value: dbServiceName},
			apiv1.EnvVar{Name: "DB_NAME", Value: dbName},
			apiv1.EnvVar{Name: "DB_USER", Value: "plugin_user"},
			apiv1.EnvVar{Name: "DB_PASSWORD", Value: "plugin_password"},
		)
	}

	// Add init container to deployment
	deployment.Spec.Template.Spec.InitContainers = append(deployment.Spec.Template.Spec.InitContainers, migrationInitContainer)

	// Update deployment
	_, err = deploymentsClient.Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment with init container: %w", err)
	}

	return nil
}
