package JupyterLabs

type Notebook struct {
	Name    string `json:"name"`
	Ready   string `json:"ready"`
	Status  string `json:"status"`
	Restart uint   `json:"restart"`
	Age     string `json:"age"`
}

type CreateLabRequest struct {
	Username        string `json:"userName"`
	Password        string `json:"password"`
	CPURequest      string `json:"cpuRequest"`
	GPURequest      string `json:"gpuRequest"`
	MemoryRequest   string `json:"memoryRequest"`
	CPULimit        string `json:"cpuLimit"`
	MemoryLimit     string `json:"memoryLimit"`
	DiskStorage     string `json:"diskStorage"`
	NodeSelector    string `json:"nodeSelector"`
	WorkSpaceType   string `json:"workspaceType"`
	LabspaceType    string `json:"labspaceType"`
	TemplateVersion string `json:"templateVersion"`
	TemplateBaseURL string `json:"templateUrl"`
}

type RestartLabRequest struct {
	Username      string `json:"userName"`
	Password      string `json:"password"`
	CPURequest    string `json:"cpuRequest"`
	GPURequest    string `json:"gpuRequest"`
	MemoryRequest string `json:"memoryRequest"`
	CPULimit      string `json:"cpuLimit"`
	MemoryLimit   string `json:"memoryLimit"`
	DiskStorage   string `json:"diskStorage"`
	NodeSelector  string `json:"nodeSelector"`
	WorkSpaceType string `json:"workspaceType"`
	LabspaceType  string `json:"labspaceType"`
}

type CloneNotebookRequest struct {
	Username          string   `json:"userName"`
	BaseUsername      string   `json:"baseUserName"`
	Password          string   `json:"password"`
	CPURequest        string   `json:"cpuRequest"`
	GPURequest        string   `json:"gpuRequest"`
	MemoryRequest     string   `json:"memoryRequest"`
	CPULimit          string   `json:"cpuLimit"`
	MemoryLimit       string   `json:"memoryLimit"`
	DiskStorage       string   `json:"diskStorage"`
	ModelName         string   `json:"modelname"`
	Version           string   `json:"version"`
	NodeSelector      string   `json:"nodeSelector"`
	WorkSpaceType     string   `json:"workspaceType"`
	LabspaceType      string   `json:"labspaceType"`
	SelectedArtifacts []string `json:"selectedArtifacts"`
}
