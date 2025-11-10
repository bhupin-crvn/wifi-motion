package kubeutils

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VendorConfig struct {
	InstanceTypeLabel   string
	CapacityTypeLabel   string
	NodeGroupLabel      string
	NodeSelectorPrefix  string
	GPUVendorLabel      string
	FieldSelectorPrefix string
}

var VendorConfigs = map[string]VendorConfig{
	"eks": {
		InstanceTypeLabel:   "beta.kubernetes.io/instance-type",
		CapacityTypeLabel:   "eks.amazonaws.com/capacityType",
		NodeGroupLabel:      "eks.amazonaws.com/nodegroup",
		NodeSelectorPrefix:  "type",
		GPUVendorLabel:      "nvidia.com/gpu",
		FieldSelectorPrefix: "spec.nodeName=",
	},
	"gke": {
		InstanceTypeLabel:   "node.kubernetes.io/instance-type",
		CapacityTypeLabel:   "cloud.google.com/gke-preemptible",
		NodeGroupLabel:      "cloud.google.com/gke-nodepool",
		NodeSelectorPrefix:  "type",
		GPUVendorLabel:      "nvidia.com/gpu",
		FieldSelectorPrefix: "spec.nodeName=",
	},
	"aks": {
		InstanceTypeLabel:   "node.kubernetes.io/instance-type",
		CapacityTypeLabel:   "kubernetes.azure.com/scalesetpriority",
		NodeGroupLabel:      "kubernetes.azure.com/agentpool",
		NodeSelectorPrefix:  "type",
		GPUVendorLabel:      "nvidia.com/gpu",
		FieldSelectorPrefix: "spec.nodeName=",
	},
	"minikube": {
		InstanceTypeLabel:   "minikube.k8s.io/instance-type",
		CapacityTypeLabel:   "kubernetes.io/os",
		NodeGroupLabel:      "minikube.k8s.io/version",
		NodeSelectorPrefix:  "type",
		GPUVendorLabel:      "nvidia.com/gpu",
		FieldSelectorPrefix: "spec.nodeName=",
	},
}

func (kc *KubernetesConfig) getClusterType() (string, error) {

	nodes, err := kc.Clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return "unknown", nil
	}

	for _, node := range nodes.Items {
		labels := node.GetLabels()

		if _, ok := labels["eks.amazonaws.com/nodegroup"]; ok {
			return "eks", nil
		}
		if _, ok := labels["cloud.google.com/gke-nodepool"]; ok {
			return "gke", nil
		}
		if _, ok := labels["kubernetes.azure.com/agentpool"]; ok {
			return "aks", nil
		}
		if _, ok := labels["minikube.k8s.io/version"]; ok {
			return "minikube", nil
		}

		if strings.HasPrefix(node.Spec.ProviderID, "aws://") {
			return "aws", nil
		}
		if strings.HasPrefix(node.Spec.ProviderID, "gce://") {
			return "gcp", nil
		}
		if strings.HasPrefix(node.Spec.ProviderID, "azure://") {
			return "azure", nil
		}
	}

	return "unknown", nil
}

func GetVendorConfig() (VendorConfig, error) {
	kc := NewKubernetesConfig()
	vendor, err := kc.getClusterType()
	if err != nil {
		return VendorConfig{}, err
	}

	if cfg, ok := VendorConfigs[vendor]; ok {
		return cfg, nil
	}

	return VendorConfig{}, fmt.Errorf("no config found for vendor: %s", vendor)
}
