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
	RoutePath  string `json:"routePath"`
}

var kc = utils.NewKubernetesConfig()

type PluginDeploymentsRequest struct {
	ZipURL     string `json:"zipUrl"`
	RoutePath  string `json:"routePath"`  // e.g. "my-plugin"
	PluginName string `json:"pluginName"` // e.g. "my-plugin"
}

// PluginInstallRequest represents the request payload accepted by the new
// `/plugins/install/{identifier}` endpoint. The identifier path parameter
// should match the EngineKey contained inside the plugin manifest.
type PluginInstallRequest struct {
	ZipURL              string            `json:"zipUrl"`
	ReleaseID           int64             `json:"releaseId"`
	RoutePath           string            `json:"routePath"`
	EngineKey           string            `json:"engineKey"`
	Force               bool              `json:"force"`
	OverwriteIngress    bool              `json:"overwriteIngress"`
	BackendCallbacks    map[string]string `json:"backendCallbacks"`
	ExpectedManifestSHA string            `json:"expectedManifestSha"`
	// Future consideration: accept allowlist of engines so platform API can
	// validate before attempting installation.
}

type PluginUpdateRequest struct {
	PluginInstallRequest
}

type PluginRollbackRequest struct {
	RoutePath string `json:"routePath"`
	Reason    string `json:"reason"`
	Force     bool   `json:"force"`
}

// PluginInstallResponse aggregates information returned to the client after a
// successful install or update operation.
type PluginInstallResponse struct {
	FrontendURL string `json:"frontendUrl"`
	BackendURL  string `json:"backendUrl"`
	Namespace   string `json:"namespace"`
	Deployment  string `json:"deployment"`
	ReleaseID   int64  `json:"releaseId"`
	EngineKey   string `json:"engineKey"`
	Version     string `json:"version"`
}
