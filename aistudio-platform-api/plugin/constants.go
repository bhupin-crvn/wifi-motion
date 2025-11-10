package plugin

import "os"

const (
	pluginNamespace = "plugin"
	ingressName     = "multi-service-ingress"
)

var (
	defaultNodeSelector = envOrDefault("PLUGIN_NODE_SELECTOR", "minikube")
	zipBaseURL          = os.Getenv("PLUGIN_ZIP_BASE_URL")
	zipEndpoint         = envOrDefault("PLUGIN_ZIP_ENDPOINT", "/api/plugins/releases/download")
)

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
