package plugin

import "os"

const (
	pluginNamespace = "plugin"
	ingressName     = "multi-service-ingress"
	NodeSelector    = "minikube"
)

var (
	ZipURL      = os.Getenv("PLUGIN_ZIP_BASE_URL")
	ZipEndpoint = os.Getenv("PLUGIN_ZIP_ENDPOINT")
)
