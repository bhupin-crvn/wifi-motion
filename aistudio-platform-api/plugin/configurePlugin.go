package plugin

import (
	"Kubernetes-api/helper"
	utils "Kubernetes-api/kubeutils"
	"fmt"
	"os"
	"path/filepath"
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
