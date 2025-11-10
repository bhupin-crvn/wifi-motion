package deployments

import (
	"Kubernetes-api/artifacts"
	"Kubernetes-api/helper"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	utils "Kubernetes-api/kubeutils"

	"github.com/gofiber/fiber/v2/log"
	apiv1 "k8s.io/api/core/v1"
)

func CreateModelDeployments(userName string, deploymentName string, Modelname string, Version string, Modelartifacts []string, cpuRequest string, gpuRequest string, memoryRequest string, cpuLimit string, memoryLimit string, diskStorage string, noddeSelector string) (string, error) {

	modelPort := 9000
	// Image := "9861531522/global-deployment:v0.3" 
	Image := "9861531522/custom-script-deployment:v1.1"
	// Image := "9861531522/custom-deployment-python3.8:v0.1" // This the container with python version 3.7
	gpuSize, err := strconv.Atoi(gpuRequest)
	if err != nil {
		log.Error(err.Error(), " Gpu value souldnot be in decimal")
		return "Gpu value souldnot be in decimal", err
	}
	cpuAvailable, err := kc.CheckCpuAvailability(cpuRequest)
	if !cpuAvailable {
		return "requested cpu is not available in any node", err
	}
	if int32(gpuSize) > 0 {
		gpuAvailable, err := kc.CheckGpuAvailability(gpuRequest)
		if !gpuAvailable {
			return "requested gpu is not available in any node", err
		}
	}
	memoryAvailable, err := kc.CheckMemoryAvailability(memoryRequest)
	if !memoryAvailable {
		return "Requested memory is not available in any node", err
	}
	deploymentName = strings.Replace(deploymentName, ".", "-", -1)
	Version = strings.Replace(Version, ".", "-", -1)
	envVars := []apiv1.EnvVar{{
		Name:  "MODELNAME",
		Value: Modelname,
	}, {
		Name:  "VERSION",
		Value: Version,
	}, {
		Name:  "MY_WORKDIR",
		Value: Modelname + Version,
	},
	}

	serviceName := deploymentName
	pvcName := fmt.Sprintf("pvc-%s", deploymentName)
	kc.CreateNamespace(modelNamespace)
	resource := utils.ConfigResource(cpuRequest, memoryRequest, cpuLimit, memoryLimit)
	if !kc.PersistentVolumeExists(modelNamespace, pvcName) {
		kc.CreatePersistentVolume(modelNamespace, pvcName, diskStorage)
		fmt.Println("Persistent volume of name is created", pvcName)
	}
	// resultCopy, errCopy := artifacts.CopyModelFile(userName, Modelname, Version, Modelartifacts)
	resultCopy, errCopy := artifacts.CopyModelArtifactsFiles(userName, deploymentName, Modelname, Version, Modelartifacts)
	if errCopy != nil {
		return "copy Artifacts fails", errCopy
	}
	if resultCopy {
		if kc.ModelDeploymentExists(modelNamespace, deploymentName) {
			kc.DeleteDeployment(modelNamespace, deploymentName)
			time.Sleep(2 * time.Second)
		}
		for kc.ModelDeploymentExists(modelNamespace, deploymentName) {
			time.Sleep(1 * time.Second)
		}
		if !kc.ServiceExists(modelNamespace, serviceName) {
			kc.CreateService(modelNamespace, serviceName, deploymentName, modelPort, apiv1.ServiceTypeClusterIP)
		}
		kc.ConfigModelDeployment(modelNamespace, deploymentName, Image, pvcName, gpuSize, modelPort, noddeSelector, resource, envVars)
		url := "http://" + deploymentName + "." + modelNamespace
		return url, nil
	}
	return "", errors.New("failed to copy model file")
}

func CreateLLMDeployments(userName string, deploymentName string, Modelname string, Version string, template string, Modelartifacts []string, cpuRequest string, gpuRequest string, memoryRequest string, cpuLimit string, memoryLimit string, diskStorage string, noddeSelector string) (string, error) {

	modelPort := 8000
	Image := helper.LllmDeploymentImage
	gpuSize, err := strconv.Atoi(gpuRequest)
	if err != nil {
		log.Error(err.Error(), " Gpu value souldnot be in decimal")
		return "Gpu value souldnot be in decimal", err
	}
	cpuAvailable, err := kc.CheckCpuAvailability(cpuRequest)
	if !cpuAvailable {
		return "requested cpu is not available in any node", err
	}
	if int32(gpuSize) > 0 {
		gpuAvailable, err := kc.CheckGpuAvailability(gpuRequest)
		if !gpuAvailable {
			return "requested gpu is not available in any node", err
		}
	}
	memoryAvailable, err := kc.CheckMemoryAvailability(memoryRequest)
	if !memoryAvailable {
		return "Requested memory is not available in any node", err
	}
	deploymentName = strings.Replace(deploymentName, ".", "-", -1)
	Version = strings.Replace(Version, ".", "-", -1)
	envVars := []apiv1.EnvVar{{
		Name:  "MODELNAME",
		Value: Modelname,
	}, {
		Name:  "VERSION",
		Value: Version,
	}, {
		Name:  "MY_WORKDIR",
		Value: Modelname + Version,
	},
	}

	serviceName := deploymentName
	pvcName := fmt.Sprintf("pvc-%s", deploymentName)
	kc.CreateNamespace(modelNamespace)
	resource := utils.ConfigResource(cpuRequest, memoryRequest, cpuLimit, memoryLimit)
	if !kc.PersistentVolumeExists(modelNamespace, pvcName) {
		kc.CreatePersistentVolume(modelNamespace, pvcName, diskStorage)
		fmt.Println("Persistent volume of name is created", pvcName)
	}
	// resultCopy, errCopy := artifacts.CopyModelFile(userName, Modelname, Version, Modelartifacts)
	resultCopy, errCopy :=  artifacts.CopyModelArtifactsFiles(userName, deploymentName, Modelname, Version, Modelartifacts)
	if errCopy != nil {
		return "copy Artifacts fails", errCopy
	}
	if resultCopy {
		if kc.ModelDeploymentExists(modelNamespace, deploymentName) {
			kc.DeleteDeployment(modelNamespace, deploymentName)
			time.Sleep(2 * time.Second)
		}
		for kc.ModelDeploymentExists(modelNamespace, deploymentName) {
			time.Sleep(1 * time.Second)
		}
		if !kc.ServiceExists(modelNamespace, serviceName) {
			kc.CreateService(modelNamespace, serviceName, deploymentName, modelPort, apiv1.ServiceTypeClusterIP)
		}
		kc.ConfigModelDeployment(modelNamespace, deploymentName, Image, pvcName, gpuSize, modelPort, noddeSelector, resource, envVars)
		url := "http://" + deploymentName + "." + modelNamespace 
		return url , nil
	}
	return "", errors.New("failed to copy model file")
}

func DeleteModelDeployments(deploymentName string) error {

	serviceName := deploymentName
	pvcName := fmt.Sprintf("pvc-%s", deploymentName)
	kc.DeleteDeployment(modelNamespace, deploymentName)
	kc.DeleteService(modelNamespace, serviceName)
	kc.DeletePersistentVolume(modelNamespace, pvcName)

	return nil
}

func ListModelDeployments() ([]map[string]string, error) {
	data, err := kc.ListPods(modelNamespace)
	if err != nil || len(data) == 0 {
		return []map[string]string{}, err
	}
	return data, nil
}

func getOneDeployment(pod string) (map[string]string, error) {
	return kc.GetPodDetail(pod, modelNamespace)
}

func getPodDescription(pod string) ([]map[string]string, error) {
	return kc.GetDeploymentPodEvents(pod, modelNamespace)
}

func getModelMetrics() ([]utils.PodMetrics, error) {
	return kc.GetPodMetric(modelNamespace)
}
