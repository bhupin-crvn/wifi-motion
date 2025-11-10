package kubeutils

import (
	"flag"
	"fmt"
	"path/filepath"

	"sync"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type KubernetesConfig struct {
	Clientset     kubernetes.Interface
	MetricsClient metricsv.Interface
	DynamicClient dynamic.Interface
}

var (
	once       sync.Once
	kubeConfig *KubernetesConfig
	configErr  error
)

func NewKubernetesConfig() *KubernetesConfig {
	once.Do(func() {
		var config *rest.Config
		var err error

		config, err = rest.InClusterConfig()
		if err != nil {
			var kubeconfig *string
			if home := homedir.HomeDir(); home != "" {
				kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"),
					"(optional) absolute path to the kubeconfig file")
			} else {
				kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
			}
			flag.Parse()

			config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
			if err != nil {
				configErr = fmt.Errorf("failed to build config from flags: %w", err)
				return
			}
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			configErr = fmt.Errorf("failed to create kubernetes clientset: %w", err)
			return
		}

		metricsClient, err := metricsv.NewForConfig(config)
		if err != nil {
			configErr = fmt.Errorf("failed to create metrics client: %w", err)
			return
		}
		dynamicClientset, err := dynamic.NewForConfig(config)
		if err != nil {
			configErr = fmt.Errorf("failed to create dynamic client: %w", err)
			return
		}

		kubeConfig = &KubernetesConfig{
			Clientset:     clientset,
			MetricsClient: metricsClient,
			DynamicClient: dynamicClientset,
		}
	})
	return kubeConfig
}
