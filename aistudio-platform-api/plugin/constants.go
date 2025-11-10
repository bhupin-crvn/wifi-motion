package plugin

const (
	pluginNamespace  = "plugin"
	ingressName      = "multi-service-ingress"
	NodeSelector     = "minikube"
	ZipURL           = "https://api.example.com" // TODO: Replace with actual API URL
	ZipEndpoint      = "/api/v1/releases/%d/download"
	storageClassName = "nfs-csi-model"
)
