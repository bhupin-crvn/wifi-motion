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
	EngineKey  string `json:"engineKey"`
	ReleaseId  int    `json:"releaseId"`
}

// PluginManifest represents the plugin manifest structure
type PluginManifest struct {
	Name     string `json:"name"`
	EngineKey string `json:"engine_key"`
	Docker   struct {
		Frontend struct {
			Image string `json:"image"`
		} `json:"frontend"`
		Backend struct {
			Image     string `json:"image"`
			Migration bool   `json:"migration"`
		} `json:"backend"`
	} `json:"docker"`
	Database struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Username string `json:"username"`
		Password string `json:"password"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
	} `json:"database"`
}