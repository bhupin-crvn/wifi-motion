package plugin

import (
	utils "Kubernetes-api/kubeutils"
)

type PluginDeploymentsRequests struct {
	// Username      string `json:"userName"`
	PluginName string `json:"pluginName"`
	// ServiceName   string `json:"serviceName"`
	Version string `json:"version"`
	// CPURequest    string `json:"cpuRequest"`
	// GPURequest    string `json:"gpuRequest"`
	// MemoryRequest string `json:"memoryRequest"`
	// CPULimit      string `json:"cpuLimit"`
	// MemoryLimit   string `json:"memoryLimit"`
	// DiskStorage   string `json:"diskStorage"`
	// NodeSelector  string `json:"nodeSelector"`
	FrontEndImage string `json:"frontEndImage"`
	BackEndImage  string `json:"backEndImage"`
	RoutePath     string `json:"routePath"`
}

type DeletePluginRequest struct {
	PluginName string `json:"pluginName"`
	RoutePath     string `json:"routePath"`
}

var kc = utils.NewKubernetesConfig()

type PluginDeploymentsRequest struct {
	ZipURL     string `json:"zipUrl"`
	RoutePath  string `json:"routePath"`  // e.g. "my-plugin"
	PluginName string `json:"pluginName"` // e.g. "my-plugin"
}