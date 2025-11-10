package plugin

type DeletePluginRequest struct {
	PluginName string `json:"pluginName"`
	EngineKey  string `json:"engineKey"`
	RoutePath  string `json:"routePath"`
}

type PluginDeploymentsRequest struct {
	EngineKey    string `json:"engineKey"`
	PluginName   string `json:"pluginName"`
	ReleaseId    int64  `json:"releaseId"`
	RoutePath    string `json:"routePath"`
	DownloadURL  string `json:"downloadUrl"`
	ZipURL       string `json:"zipUrl"`
	NodeSelector string `json:"nodeSelector"`
}

type PluginInstallResult struct {
	FrontendURL       string
	BackendURL        string
	Namespace         string
	ManifestPath      string
	ArtifactsLocation string
}
