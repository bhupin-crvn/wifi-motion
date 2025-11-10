package JupyterLabs

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	apiv1 "k8s.io/api/core/v1"

	"Kubernetes-api/artifacts"
	"Kubernetes-api/helper"
	"Kubernetes-api/kubeutils"
)

var kc = kubeutils.NewKubernetesConfig()

type DeleteNotebookRequest struct {
	Username string `json:"userName"`
}

func CreateNotebook(userName, password, cpuRequest, gpuRequest, memoryRequest, cpuLimit, memoryLimit, diskStorage, nodeSelector, labType, aiType string) (string, error) {
	gpuSize, err := strconv.Atoi(gpuRequest)
	if err != nil {
		logrus.Errorf("invalid GPU request value: %s, error: %v", gpuRequest, err)
		return "", fmt.Errorf("GPU value must be an integer: %w", err)
	}

	if gpuSize > 0 {
		if available, err := kc.CheckGpuAvailability(gpuRequest); err != nil || !available {
			logrus.Errorf("GPU check failed: %v", err)
			return "", fmt.Errorf("requested GPU is not available: %w", err)
		}
	}

	if available, err := kc.CheckMemoryAvailability(memoryRequest); err != nil || !available {
		logrus.Errorf("memory check failed: %v", err)
		return "", fmt.Errorf("requested memory is not available: %w", err)
	}

	if available, err := kc.CheckCpuAvailability(cpuRequest); err != nil || !available {
		logrus.Errorf("CPU check failed: %v", err)
		return "", fmt.Errorf("requested CPU is not available: %w", err)
	}

	envVars := []apiv1.EnvVar{
		{Name: EnvNotebookUser, Value: userName},
		{Name: EnvPassword, Value: password},
		{Name: EnvGrantSudo, Value: "yes"},
		{Name: EnvJupyterEnableLab, Value: "yes"},
		{Name: EnvNbUID, Value: "1000"},
		{Name: EnvNbGID, Value: "1000"},
		{Name: EnvExperimentName, Value: userName},
	}

	kc.CreateNamespace(NotebookNamespace)
	ingressRule := fmt.Sprintf("/%s", userName)
	serviceName := fmt.Sprintf("%s%s", NotebookServicePrefix, userName)
	kc.CreateService(NotebookNamespace, serviceName, userName, NotebookPort, apiv1.ServiceTypeNodePort)
	resource := kubeutils.ConfigResource(cpuRequest, memoryRequest, cpuLimit, memoryLimit)

	image := helper.CodeServerImage
	switch aiType {
	case AiTypeMLModel:
		switch labType {
		case LabTypeJupyterlab:
			image = helper.JupyterlabImage
		case LabTypeCodeServer:
			image = helper.CodeServerImage
		default:
			logrus.Warnf("unknown labType: %s, using default image", labType)
		}
		kc.CreateStatefulSet(NotebookNamespace, userName, serviceName, image, gpuSize, NotebookPort, diskStorage, nodeSelector, resource, envVars)
		kc.AppendRuleToIngress(NotebookNamespace, labIngress, serviceName, ingressRule)
	case AiTypeAgent:
		adkIngressRuleFrontend := fmt.Sprintf("/%s%s", userName, AdkIngressFrontendSuffix)
		adkIngressRuleBackend := fmt.Sprintf("/%s%s", userName, AdkIngressBackendSuffix)
		kc.AppendRuleToIngress(NotebookNamespace, labIngress, serviceName, ingressRule)
		imagecodeserver := helper.AgentCodeServerImage
		imageAdk := helper.ADKUIImage
		envVarsAdk := []apiv1.EnvVar{
			{Name: EnvNotebookUser, Value: userName},
			{Name: EnvPassword, Value: password},
			{Name: FrontEndPath, Value: adkIngressRuleFrontend},
			{Name: FrontEndDomain, Value: WorkSpaceDomain},
		}
		kc.AppendRuleToIngress(NotebookNamespace, labIngress, serviceName, adkIngressRuleFrontend)
		env := [][]apiv1.EnvVar{envVars, envVarsAdk}
		kc.CreateStatefulSetWithDualContainer(NotebookNamespace, userName, serviceName, imagecodeserver, imageAdk, gpuSize, NotebookPort, AdkPort, diskStorage, nodeSelector, resource, env)
		kc.AppendRuleToIngress(NotebookNamespace, labIngress, serviceName, adkIngressRuleBackend)

	default:
		logrus.Warnf("unknown aiType: %s, using default image", aiType)
		kc.CreateStatefulSet(NotebookNamespace, userName, serviceName, image, gpuSize, NotebookPort, diskStorage, nodeSelector, resource, envVars)
		kc.AppendRuleToIngress(NotebookNamespace, labIngress, serviceName, ingressRule)
	}

	return "Notebook created successfully", nil
}

func DeleteNotebook(userName string) error {
	pvcName := fmt.Sprintf("%s%s%s", PersistentVolumePrefix, userName, PersistentVolumeSuffix)
	if kc.PersistentVolumeExists(NotebookNamespace, pvcName) {
		if err := kc.DeletePersistentVolume(NotebookNamespace, pvcName); err != nil {
			logrus.Errorf("failed to delete persistent volume %s: %v", pvcName, err)
		}
	}

	serviceName := fmt.Sprintf("%s%s", NotebookServicePrefix, userName)
	adkServiceName := fmt.Sprintf("%s%s", AdkServicePrifix, userName)
	adkIngressRule := fmt.Sprintf("%s%s", userName, AdkIngressSuffix)
	kc.DeleteService(NotebookNamespace, serviceName)
	if kc.ServiceExists(NotebookNamespace, adkServiceName) {
		kc.DeleteService(NotebookNamespace, adkServiceName)
		kc.DeleteRuleFromIngress(NotebookNamespace, adkIngressRule, labIngress)
	}
	kc.DeleteStatefulSet(NotebookNamespace, userName)
	kc.DeleteRuleFromIngress(NotebookNamespace, userName, labIngress)
	return nil
}
func StopNotebook(userName string) error {

	serviceName := fmt.Sprintf("%s%s", NotebookServicePrefix, userName)
	adkServiceName := fmt.Sprintf("%s%s", AdkServicePrifix, userName)
	adkIngressRule := fmt.Sprintf("%s%s", userName, AdkIngressSuffix)
	kc.DeleteService(NotebookNamespace, serviceName)
	if kc.ServiceExists(NotebookNamespace, adkServiceName) {
		kc.DeleteService(NotebookNamespace, adkServiceName)
		kc.DeleteRuleFromIngress(NotebookNamespace, adkIngressRule, labIngress)
	}
	kc.DeleteStatefulSet(NotebookNamespace, userName)
	kc.DeleteRuleFromIngress(NotebookNamespace, userName, labIngress)
	return nil
}

func ListNotebooks() ([]map[string]string, error) {
	return kc.ListPods(NotebookNamespace)
}

func GetOneNotebook(notebook string) (map[string]string, error) {
	return kc.GetPodDetail(notebook, NotebookNamespace)
}

func CloneArtifactsNotebook(req CloneNotebookRequest) (string, error) {

	fs := afero.NewOsFs()
	normalizedVersion := strings.ReplaceAll(req.Version, ".", "-")
	src := fmt.Sprintf("%s%s/%s-%s", ModelRegistryPathPrefix, req.BaseUsername, req.ModelName, normalizedVersion)

	if exists, err := afero.DirExists(fs, src); err != nil {
		log.Error("error checking source directory: ", err)
		return "", fmt.Errorf("error checking source directory: %w", err)
	} else if !exists {
		log.Error("source directory does not exist: ", src)
		return "", fmt.Errorf("source directory does not exist: %s", src)
	}

	message, err := CreateNotebook(
		req.Username, req.Password, req.CPURequest, req.GPURequest, req.MemoryRequest,
		req.CPULimit, req.MemoryLimit, req.DiskStorage, req.NodeSelector,
		req.WorkSpaceType, req.LabspaceType,
	)

	if err != nil {
		log.Error("error creating notebook: ", err)
		return "", fmt.Errorf("error creating notebook: %w", err)
	}

	time.Sleep(6 * time.Second)
	dst := fmt.Sprintf("%s%s%s", ArtifactPathPrefix, PersistentVolumePrefix, req.Username+PersistentVolumeSuffix)

	if exists, err := afero.DirExists(fs, dst); err != nil {
		log.Error("error checking destination directory: ", err)
		return "", fmt.Errorf("error checking destination directory: %w", err)
	} else if exists {
		if _, err := artifacts.CloneSelectedArtifacts(src, dst, req.SelectedArtifacts); err != nil {

			if delErr := DeleteNotebook(req.Username); delErr != nil {
				logrus.Errorf("failed to clean up notebook after artifact cloning error: %v", delErr)
			}
			return "", fmt.Errorf("error cloning artifacts: %w", err)
		}
	}

	return message, nil
}

func GetLabspacesMetrics() ([]kubeutils.PodMetrics, error) {
	return kc.GetPodMetric(NotebookNamespace)
}
