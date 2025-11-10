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
)

// parseManifest reads and parses the plugin manifest from extracted files
func parseManifest(extractedFiles []string) (*PluginManifest, string, error) {
	var manifestPath string
	
	// Find manifest.json in extracted files
	for _, file := range extractedFiles {
		if strings.HasSuffix(file, "manifest.json") || strings.HasSuffix(file, "plugin.json") {
			manifestPath = file
			break
		}
	}
	
	if manifestPath == "" {
		return nil, "", fmt.Errorf("manifest.json not found in plugin archive")
	}
	
	// Read manifest file
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read manifest: %w", err)
	}
	
	// Parse JSON
	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, "", fmt.Errorf("failed to parse manifest JSON: %w", err)
	}
	
	return &manifest, manifestPath, nil
}

// InstallPlugin installs a plugin with database and migration support
func InstallPlugin(req InstallPluginRequest) (string, error) {
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
	apiURL := fmt.Sprintf("%s%s", ZipURL, ZipEndpoint)
	if err := helper.DownloadFileWithReleaseId(req.ReleaseId, apiURL, zipFilePath); err != nil {
		return "", fmt.Errorf("failed to download plugin zip: %w", err)
	}
	
	// Clean up zip file after extraction
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
	// Get PVC size from manifest or use default
	diskStorage := "10Gi"
	if manifest.Storage.PVCSize != "" {
		diskStorage = manifest.Storage.PVCSize
	}
	pvcName := fmt.Sprintf("%s-data", req.EngineKey)

	if err := kc.CreatePersistentVolume(pluginNamespace, pvcName, diskStorage, storageClassName); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return "", fmt.Errorf("failed to create PVC: %w", err)
		}
	}

	// === 10. Copy artifacts to destination ===
	result, err := artifacts.CopyAllArtifacts(extractDir, destinationDir, true)
	if err != nil {
		return "", fmt.Errorf("failed to copy artifacts: %w", err)
	}
	fmt.Println("Files copied from source to destination for backend:", result)

	// === 11. Database Setup (if enabled) ===
	var initContainers []apiv1.Container
	if manifest.Database.Enabled {
		// TODO: Create database with access credentials
		// This would typically involve:
		// 1. Creating a database instance (or using existing one)
		// 2. Creating database user with proper permissions
		// 3. Running initial schema setup
		
		dbHost := os.Getenv("DB_HOST")
		if dbHost == "" {
			dbHost = "postgres-service.default.svc.cluster.local" // Default K8s service
		}
		
		// Add init container for database migration
		if manifest.Docker.Backend.Migration {
			migrationContainer := apiv1.Container{
				Name:  "migration",
				Image: manifest.Docker.Backend.Image,
				Command: []string{
					"/bin/sh",
					"-c",
					"echo 'Running database migrations...'; /app/migrate || (echo 'Migration failed, reverting...' && /app/migrate-rollback && exit 1)",
				},
				Env: []apiv1.EnvVar{
					{Name: "DB_HOST", Value: dbHost},
					{Name: "DB_NAME", Value: manifest.Database.Name},
					{Name: "DB_USER", Value: manifest.Database.User},
					{Name: "DB_PASSWORD", Value: manifest.Database.Password},
					{Name: "MIGRATION_MODE", Value: "up"},
				},
			}
			initContainers = append(initContainers, migrationContainer)
		}
	}

	// === 12. Prepare environment variables ===
	envVars := []apiv1.EnvVar{
		{Name: "PLUGIN_NAME", Value: manifest.Name},
		{Name: "PLUGIN_VERSION", Value: manifest.Version},
		{Name: "ENGINE_KEY", Value: manifest.EngineKey},
	}
	
	// Add custom environment variables from manifest
	for key, value := range manifest.Environment {
		envVars = append(envVars, apiv1.EnvVar{Name: key, Value: value})
	}
	
	// Add database environment variables if enabled
	if manifest.Database.Enabled {
		dbHost := os.Getenv("DB_HOST")
		if dbHost == "" {
			dbHost = "postgres-service.default.svc.cluster.local"
		}
		envVars = append(envVars,
			apiv1.EnvVar{Name: "DB_HOST", Value: dbHost},
			apiv1.EnvVar{Name: "DB_NAME", Value: manifest.Database.Name},
			apiv1.EnvVar{Name: "DB_USER", Value: manifest.Database.User},
			apiv1.EnvVar{Name: "DB_PASSWORD", Value: manifest.Database.Password},
		)
	}

	// === 13. Node selector configuration ===
	nodeSelector := NodeSelector // Use default from constants

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
		fmt.Printf("Frontend deployment %s created successfully\n", frontendDeploymentName)
	} else {
		fmt.Printf("Frontend deployment %s already exists, skipping creation\n", frontendDeploymentName)
	}

	// === 15. Create Backend Deployment ===
	if !kc.DeploymentExists(pluginNamespace, backendDeploymentName) {
		backendEnv := append(envVars, apiv1.EnvVar{
			Name: "MIGRATION_ENABLED", Value: strconv.FormatBool(manifest.Docker.Backend.Migration),
		})

		// Setup volumes for PVC
		volumes := []apiv1.Volume{
			{
				Name: pvcName,
				VolumeSource: apiv1.VolumeSource{
					PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			},
		}
		
		volumeMounts := []apiv1.VolumeMount{
			{
				Name:      pvcName,
				MountPath: "/data",
			},
		}

		// Create deployment with or without init containers
		if len(initContainers) > 0 {
			// Use the function with init containers support
			_, err := kc.ConfigDeploymentWithInitContainers(
				pluginNamespace,
				backendDeploymentName,
				manifest.Docker.Backend.Image,
				8080, // backend port
				nodeSelector,
				backendEnv,
				initContainers,
				volumes,
				volumeMounts,
			)
			if err != nil {
				return "", fmt.Errorf("failed to create backend deployment with init containers: %w", err)
			}
			fmt.Printf("Backend deployment %s created successfully with init containers\n", backendDeploymentName)
		} else {
			// Use the standard function without init containers
			_, err := kc.ConfigDeployment(
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
			fmt.Printf("Backend deployment %s created successfully\n", backendDeploymentName)
		}
	} else {
		fmt.Printf("Backend deployment %s already exists, skipping creation\n", backendDeploymentName)
	}

	// === 16. Create Services ===
	port := 80
	if !kc.ServiceExists(pluginNamespace, frontendServiceName) {
		kc.CreateService(pluginNamespace, frontendServiceName, frontendDeploymentName, port, apiv1.ServiceTypeClusterIP)
		fmt.Printf("Frontend service %s created successfully\n", frontendServiceName)
	} else {
		fmt.Printf("Frontend service %s already exists, skipping creation\n", frontendServiceName)
	}
	
	if !kc.ServiceExists(pluginNamespace, backendServiceName) {
		kc.CreateService(pluginNamespace, backendServiceName, backendDeploymentName, 8080, apiv1.ServiceTypeClusterIP)
		fmt.Printf("Backend service %s created successfully\n", backendServiceName)
	} else {
		fmt.Printf("Backend service %s already exists, skipping creation\n", backendServiceName)
	}

	// === 17. Update Ingress ===
	ingressName := "aistudio-ingress" // Use from constants or make configurable
	frontendPath := fmt.Sprintf("/plugins/%s", req.EngineKey)
	backendPath := fmt.Sprintf("/plugins/%s/api", req.EngineKey)

	if err := kc.AppendRuleToIngress(pluginNamespace, ingressName, frontendServiceName, frontendPath); err != nil {
		fmt.Printf("Warning: failed to add frontend ingress rule: %v\n", err)
	}
	if err := kc.AppendRuleToIngress(pluginNamespace, ingressName, backendServiceName, backendPath); err != nil {
		fmt.Printf("Warning: failed to add backend ingress rule: %v\n", err)
	}

	// === 18. Construct URLs ===
	// TODO: Get actual domain from configuration or ingress
	domain := os.Getenv("INGRESS_DOMAIN")
	if domain == "" {
		domain = "localhost" // Default
	}
	
	frontendURL := fmt.Sprintf("http://%s%s", domain, frontendPath)
	backendURL := fmt.Sprintf("http://%s%s", domain, backendPath)

	// === 19. Clean up temporary extraction directory ===
	defer os.RemoveAll(extractDir)

	// === 20. Return success message ===
	message := fmt.Sprintf(
		"Plugin '%s' (v%s) installed successfully!\n"+
			"Frontend: %s\n"+
			"Backend: %s\n"+
			"Namespace: %s\n"+
			"Database: %v",
		manifest.Name,
		manifest.Version,
		frontendURL,
		backendURL,
		pluginNamespace,
		manifest.Database.Enabled,
	)
	
	return message, nil
}
