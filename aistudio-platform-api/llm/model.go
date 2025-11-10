package llm

import (
	utils "Kubernetes-api/kubeutils"
)


var modelNamespace = "model"

type CreateLlmDeploymentsRequest struct {
	DeploymentName string `json:"deploymentName"`
	Modelname      string `json:"modelName"`
	CPURequest     string `json:"cpuRequest"`
	GPURequest     string `json:"gpuRequest"`
	MemoryRequest  string `json:"memoryRequest"`
	CPULimit       string `json:"cpuLimit"`
	MemoryLimit    string `json:"memoryLimit"`
	DiskStorage    string `json:"diskStorage"`
	NodeSelector   string `json:"nodeSelector"`
	BackendTpye    string `json:"backendType"`
}

var kc = utils.NewKubernetesConfig()
