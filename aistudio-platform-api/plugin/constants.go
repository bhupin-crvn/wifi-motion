package plugin

const (
	pluginNamespace  = "plugin"
	ingressName      = "multi-service-ingress"
	NodeSelector     = "minikube"
	storageClassName = "nfs-csi-model"
	ZipURL           = "https://api.example.com" // TODO: Configure from environment
	ZipEndpoint      = "/releases/download"       // TODO: Configure from environment
)
