package plugin

const (
	pluginNamespace  = "plugin"
	ingressName      = "aistudio-ingress"
	NodeSelector     = "gpu"
	ZipURL           = "https://api.example.com" // TODO: Replace with actual API URL
	ZipEndpoint      = "/api/plugins/download"
	storageClassName = "nfs-csi-model"
)
