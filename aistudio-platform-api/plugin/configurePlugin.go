package plugin

import (
	"Kubernetes-api/artifacts"
	"Kubernetes-api/helper"
	utils "Kubernetes-api/kubeutils"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"
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

// parseManifest parses the plugin manifest file from extracted files
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

// InstallPlugin installs a plugin from a release ID
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
	apiURL := fmt.Sprintf("%s%s", ZipURL, fmt.Sprintf(ZipEndpoint, req.ReleaseId))
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
	fmt.Println("File copied from source to destination for backend", result)

	// === 11. Prepare init containers for database migration (if needed) ===
	var initContainers []apiv1.Container
	if manifest.Docker.Backend.Migration && manifest.Database.Type != "" {
		var err error
		initContainers, err = createDatabaseAndRunMigration(kc, pluginNamespace, backendDeploymentName, manifest, pvcName)
		if err != nil {
			return "", fmt.Errorf("failed to setup database and migration: %w", err)
		}
	}

	// === 12. Prepare environment variables ===
	envVars := []apiv1.EnvVar{
		{Name: "PLUGIN_NAME", Value: manifest.Name},
	}

	// Add database environment variables if database is configured
	if manifest.Database.Type != "" {
		envVars = append(envVars,
			apiv1.EnvVar{Name: "DB_TYPE", Value: manifest.Database.Type},
			apiv1.EnvVar{Name: "DB_NAME", Value: manifest.Database.Name},
			apiv1.EnvVar{Name: "DB_USER", Value: manifest.Database.Username},
			apiv1.EnvVar{Name: "DB_PASSWORD", Value: manifest.Database.Password},
			apiv1.EnvVar{Name: "DB_HOST", Value: manifest.Database.Host},
			apiv1.EnvVar{Name: "DB_PORT", Value: strconv.Itoa(manifest.Database.Port)},
		)
	}

	// === 13. Node selector (optional) ===
	nodeSelector := NodeSelector // Use constant from config

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

	// === 15. Create Backend Deployment ===
	if !kc.DeploymentExists(pluginNamespace, backendDeploymentName) {
		backendEnv := append(envVars, apiv1.EnvVar{
			Name: "MIGRATION_ENABLED", Value: strconv.FormatBool(manifest.Docker.Backend.Migration),
		})

		// Add PVC volume mount for backend if migrations are enabled
		var backendInitContainers []apiv1.Container
		var volumes []apiv1.Volume
		if len(initContainers) > 0 {
			backendInitContainers = initContainers
			// Add PVC volume for init container to access migration scripts
			volumes = []apiv1.Volume{
				{
					Name: pvcName,
					VolumeSource: apiv1.VolumeSource{
						PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			}
		}

		_, err := kc.ConfigDeploymentWithInitContainersAndVolumes(
			pluginNamespace,
			backendDeploymentName,
			manifest.Docker.Backend.Image,
			8080, // backend port
			nodeSelector,
			backendEnv,
			backendInitContainers,
			volumes,
		)
		if err != nil {
			return "", fmt.Errorf("failed to create backend deployment: %w", err)
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
		return "", fmt.Errorf("failed to add frontend ingress rule: %w", err)
	}
	if err := kc.AppendRuleToIngress(pluginNamespace, ingressName, backendServiceName, backendPath); err != nil {
		return "", fmt.Errorf("failed to add backend ingress rule: %w", err)
	}

	// === 18. Construct URLs ===
	frontendURL := fmt.Sprintf("%s", frontendPath) // TODO: Use real domain
	backendURL := fmt.Sprintf("%s", backendPath)

	// === 19. Return success message ===
	message := fmt.Sprintf(
		"Plugin installed successfully!\nFrontend: %s\nBackend: %s\nNamespace: %s",
		frontendURL, backendURL, pluginNamespace,
	)
	return message, nil
}

// createDatabaseAndRunMigration creates an init container configuration for database setup and migration
func createDatabaseAndRunMigration(kc *utils.KubernetesConfig, namespace, deploymentName string, manifest *PluginManifest, pvcName string) ([]apiv1.Container, error) {
	// This function creates an init container that:
	// 1. Creates the database if it doesn't exist
	// 2. Runs database migrations
	// 3. Exits with error if migration fails (which will prevent the main container from starting)

	fmt.Printf("Setting up database %s and running migrations for deployment %s\n", manifest.Database.Name, deploymentName)

	var initContainer apiv1.Container
	var migrationImage string
	var migrationCommand []string

	// Determine migration image and command based on database type
	switch manifest.Database.Type {
	case "postgresql", "postgres":
		migrationImage = "postgres:15-alpine"
		// Create database if it doesn't exist, then run migrations
		migrationCommand = []string{
			"sh",
			"-c",
			fmt.Sprintf(
				"PGPASSWORD=\"$DB_PASSWORD\" psql -h \"$DB_HOST\" -p \"$DB_PORT\" -U \"$DB_USER\" -d postgres -tc \"SELECT 1 FROM pg_database WHERE datname='%s'\" | grep -q 1 || PGPASSWORD=\"$DB_PASSWORD\" psql -h \"$DB_HOST\" -p \"$DB_PORT\" -U \"$DB_USER\" -d postgres -c \"CREATE DATABASE \\\"%s\\\";\" && PGPASSWORD=\"$DB_PASSWORD\" psql -h \"$DB_HOST\" -p \"$DB_PORT\" -U \"$DB_USER\" -d \"%s\" -f /migrations/migrate.sql || exit 1",
				manifest.Database.Name, manifest.Database.Name, manifest.Database.Name,
			),
		}
	case "mysql", "mariadb":
		migrationImage = "mysql:8.0"
		migrationCommand = []string{
			"sh",
			"-c",
			fmt.Sprintf(
				"mysql -h \"$DB_HOST\" -P \"$DB_PORT\" -u \"$DB_USER\" -p\"$DB_PASSWORD\" -e \"CREATE DATABASE IF NOT EXISTS \\`%s\\`;\" && mysql -h \"$DB_HOST\" -P \"$DB_PORT\" -u \"$DB_USER\" -p\"$DB_PASSWORD\" \"%s\" < /migrations/migrate.sql || exit 1",
				manifest.Database.Name, manifest.Database.Name,
			),
		}
	default:
		return nil, fmt.Errorf("unsupported database type: %s", manifest.Database.Type)
	}

	initContainer = apiv1.Container{
		Name:    "db-migration",
		Image:   migrationImage,
		Command: migrationCommand,
		Env: []apiv1.EnvVar{
			{Name: "DB_TYPE", Value: manifest.Database.Type},
			{Name: "DB_NAME", Value: manifest.Database.Name},
			{Name: "DB_USER", Value: manifest.Database.Username},
			{Name: "DB_PASSWORD", Value: manifest.Database.Password},
			{Name: "DB_HOST", Value: manifest.Database.Host},
			{Name: "DB_PORT", Value: strconv.Itoa(manifest.Database.Port)},
		},
		// Mount the PVC to access migration scripts
		VolumeMounts: []apiv1.VolumeMount{
			{
				Name:      pvcName,
				MountPath: "/migrations",
			},
		},
	}

	return []apiv1.Container{initContainer}, nil
}
