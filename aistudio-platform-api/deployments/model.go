package deployments

import (
	utils "Kubernetes-api/kubeutils"
)

type Model struct {
	Name    string `json:"name"`
	Ready   string `json:"ready"`
	Status  string `json:"status"`
	Restart uint   `json:"restart"`
	Age     string `json:"age"`
}

type CreateModelDeploymentsRequest struct {
	Username       string   `json:"userName"`
	DeploymentName string   `json:"deploymentName"`
	Modelname      string   `json:"modelName"`
	Version        string   `json:"version"`
	Modelartifacts []string `json:"modelartifacts"`
	CPURequest     string   `json:"cpuRequest"`
	GPURequest     string   `json:"gpuRequest"`
	MemoryRequest  string   `json:"memoryRequest"`
	CPULimit       string   `json:"cpuLimit"`
	MemoryLimit    string   `json:"memoryLimit"`
	DiskStorage    string   `json:"diskStorage"`
	NodeSelector   string   `json:"nodeSelector"`
}

type EnvVar struct {
	Key   string
	Value string
}

type DeleteDeploymentRequest struct {
	Username string `json:"userName"`
}

var modelNamespace = "model"
var kc = utils.NewKubernetesConfig()
