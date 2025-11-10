package JupyterLabs

const (
	NotebookNamespace        = "lab"
	NotebookPort             = 8888
	AdkPort                  = 9005
	NotebookServicePrefix    = "notebook-"
	AdkServicePrifix         = "adkweb-"
	AdkIngressFrontendSuffix = "/adk-ui/"
	AdkIngressBackendSuffix  = "/adk/"
	AdkIngressSuffix         = "/adk/"
	PersistentVolumePrefix   = "jl-"
	PersistentVolumeSuffix   = "-0"
	labIngress               = "labs"
)

const (
	EnvNotebookUser     = "NOTEBOOK_USER"
	EnvPassword         = "PASSWORD"
	EnvGrantSudo        = "GRANT_SUDO"
	EnvJupyterEnableLab = "JUPYTER_ENABLE_LAB"
	EnvNbUID            = "NB_UID"
	EnvNbGID            = "NB_GID"
	EnvExperimentName   = "EXPERIMENT_NAME"
	FrontEndPath        = "ANGULAR_PATH"
	FrontEndDomain      = "DOMAIN_NAME"
)

const (
	LabTypeJupyterlab = "jupyterlab"
	LabTypeCodeServer = "codeserver"
	AiTypeMLModel     = "ML_MODEL_LABSPACE"
	AiTypeAgent       = "AGENT_LABSPACE"
)

const (
	ArtifactPathPrefix      = "/app/artifact/"
	ModelRegistryPathPrefix = "/app/artifact/ModelRegistry/"
)

const (
	GitTokenEnv = "git_token"
)

const (
	SSEDataPrefix = "data: %s\n\n"
)

const (
	WorkSpaceDomain = "https://devlabs.fuse.ai"
)
