package llm

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	utils "Kubernetes-api/kubeutils"

	"github.com/gofiber/fiber/v2/log"
	apiv1 "k8s.io/api/core/v1"
)

func CreateLlmDeployments(req CreateLlmDeploymentsRequest) (string, error) {

	modelPort := 8000
	Image := "9861531522/general-llm-deployment:v0.4"
	gpuSize, err := strconv.Atoi(req.GPURequest)
	if err != nil {
		log.Error(err.Error(), " Gpu value souldnot be in decimal")
		return "Gpu value souldnot be in decimal", err
	}
	cpuAvailable, err := kc.CheckCpuAvailability(req.CPURequest)
	if !cpuAvailable {
		return "requested cpu is not available in any node", err
	}
	if int32(gpuSize) > 0 {
		gpuAvailable, err := kc.CheckGpuAvailability(req.GPURequest)
		if !gpuAvailable {
			return "requested gpu is not available in any node", err
		}
	}
	memoryAvailable, err := kc.CheckMemoryAvailability(req.MemoryRequest)
	if !memoryAvailable {
		return "Requested memory is not available in any node", err
	}
	req.DeploymentName = strings.Replace(req.DeploymentName, ".", "-", -1)
	envVars := []apiv1.EnvVar{
		{
			Name:  "MODEL_NAME",
			Value: req.Modelname,
		},
		{
			Name:  "BACKEND_TYPE",
			Value: req.BackendTpye,
		},
	}
	serviceName := req.DeploymentName
	pvcName := fmt.Sprintf("pvc-%s", "llm")
	kc.CreateNamespace(modelNamespace)
	resource := utils.ConfigResource(req.CPURequest, req.MemoryRequest, req.CPULimit, req.MemoryLimit)

	if kc.ModelDeploymentExists(modelNamespace, req.DeploymentName) {
		kc.DeleteDeployment(modelNamespace, req.DeploymentName)
		time.Sleep(2 * time.Second)
	}
	for kc.ModelDeploymentExists(modelNamespace, req.DeploymentName) {
		time.Sleep(1 * time.Second)
	}
	if !kc.ServiceExists(modelNamespace, serviceName) {
		kc.CreateService(modelNamespace, serviceName, req.DeploymentName, modelPort, apiv1.ServiceTypeClusterIP)
	}
	kc.ConfigModelDeployment(modelNamespace, req.DeploymentName, Image, pvcName, gpuSize, modelPort, req.NodeSelector, resource, envVars)
	url := "http://" + req.DeploymentName + "." + modelNamespace + "/v2/models/"+ req.BackendTpye +"/generate"

	return url, nil

}

func DeleteLlmDeployments(deploymentName string) error {

	serviceName := deploymentName
	kc.DeleteDeployment(modelNamespace, deploymentName)
	kc.DeleteService(modelNamespace, serviceName)

	return nil
}

func getFolderNames(path string) ([]string, error) {
	var folderNames []string
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			folderNames = append(folderNames, entry.Name())
		}
	}

	return folderNames, nil
}

