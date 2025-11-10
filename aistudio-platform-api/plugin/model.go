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

type InstallPluginRequest struct {
	EngineKey string `json:"engineKey"` // e.g. "my-plugin-engine"
	ReleaseId int    `json:"releaseId"` // e.g. 123
}

// Manifest structure from plugin zip
type PluginManifest struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	EngineKey   string `json:"engine_key"`
	Description string `json:"description"`
	Docker      struct {
		Frontend struct {
			Image string `json:"image"`
		} `json:"frontend"`
		Backend struct {
			Image     string `json:"image"`
			Migration bool   `json:"migration"`
		} `json:"backend"`
	} `json:"docker"`
	Database struct {
		Enabled  bool   `json:"enabled"`
		Name     string `json:"name"`
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"database"`
	Storage struct {
		PVCSize string `json:"pvc_size"` // e.g. "10Gi"
	} `json:"storage"`
	Environment map[string]string `json:"environment"`
}