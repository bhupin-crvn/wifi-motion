package kubeutils

import (
	"context"
	"fmt"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeResources struct {
	CPU          string
	Memory       string
	GPU          string
	InstanceType string
	CapacityType string
	NodeGroup    string
	Type         string
	IP           string
}

type ClusterNodeResources struct {
	CPU              string
	Memory           string
	GPU              string
	AavailableCPU    string
	AavailableMemory string
	AavailableGPU    string
	Machine          string
	CapacityType     string
	NodePool         string
	Type             string
	NodeIP           string
}
type PodMetrics struct {
	PodName     string
	CPUUsage    string
	GPUUsage    string
	MemoryUsage string
}

func (kc *KubernetesConfig) GetClusterNodeResources() ([]ClusterNodeResources, error) {
	cfg, err := GetVendorConfig()
	if err != nil {
		return nil, err
	}

	ctx := context.TODO()
	nodes, err := kc.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var nodeResources []ClusterNodeResources
	for _, node := range nodes.Items {
		pods, err := kc.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			FieldSelector: cfg.FieldSelectorPrefix + node.Name,
		})
		if err != nil {
			return nil, err
		}

		instanceType := node.Labels[cfg.InstanceTypeLabel]
		capacityType := node.Labels[cfg.CapacityTypeLabel]
		nodeGroup := node.Labels[cfg.NodeGroupLabel]
		engine := node.Labels[cfg.NodeSelectorPrefix]
		ip := node.Name

		usedCPU := resource.Quantity{}
		usedMemory := resource.Quantity{}
		usedGPU := resource.Quantity{}
		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				usedCPU.Add(container.Resources.Requests[v1.ResourceCPU])
				usedMemory.Add(container.Resources.Requests[v1.ResourceMemory])
				if val, ok := container.Resources.Requests[v1.ResourceName(cfg.GPUVendorLabel)]; ok {
					usedGPU.Add(val)
				}
			}
		}

		totalCPU := node.Status.Allocatable[v1.ResourceCPU]
		cpuCoresTotal := totalCPU.MilliValue() / 1000
		totalCPU.Sub(usedCPU)
		cpuCoresRemaining := totalCPU.MilliValue() / 1000

		totalMemory := node.Status.Allocatable[v1.ResourceMemory]
		memGiBTotal := int(totalMemory.Value() / (1024 * 1024 * 1024))
		totalMemory.Sub(usedMemory)
		memGiBRemaining := int(totalMemory.Value() / (1024 * 1024 * 1024))

		totalGPU := node.Status.Allocatable[v1.ResourceName(cfg.GPUVendorLabel)]
		gpuTotal := totalGPU.Value()
		totalGPU.Sub(usedGPU)
		gpuRemaining := totalGPU.Value()

		nodeResources = append(nodeResources, ClusterNodeResources{
			CPU:              fmt.Sprintf("%d", cpuCoresTotal),
			Memory:           fmt.Sprintf("%d", memGiBTotal),
			GPU:              fmt.Sprintf("%d", gpuTotal),
			AavailableCPU:    fmt.Sprintf("%d", cpuCoresRemaining),
			AavailableMemory: fmt.Sprintf("%d", memGiBRemaining),
			AavailableGPU:    fmt.Sprintf("%d", gpuRemaining),
			Machine:          instanceType,
			CapacityType:     capacityType,
			NodePool:         nodeGroup,
			Type:             engine,
			NodeIP:           ip,
		})
	}
	return nodeResources, nil
}

func int64Ptr(i int64) *int64 { return &i }

func int32Ptr(i int32) *int32 { return &i }

func GigabytesToBytes(gigabytes int64) int64 {
	return gigabytes * 1024 * 1024 * 1024
}

func configGpu(spec *v1.PodSpec, gpuRequest string) {
	cfg, _ := GetVendorConfig()

	for i := range spec.Containers {
		if spec.Containers[i].Resources.Requests == nil {
			spec.Containers[i].Resources.Requests = v1.ResourceList{}
		}
		if spec.Containers[i].Resources.Limits == nil {
			spec.Containers[i].Resources.Limits = v1.ResourceList{}
		}
		gpuResource := resource.MustParse(gpuRequest)
		spec.Containers[i].Resources.Requests[v1.ResourceName(cfg.GPUVendorLabel)] = gpuResource
		spec.Containers[i].Resources.Limits[v1.ResourceName(cfg.GPUVendorLabel)] = gpuResource
	}
}

func ConfigResource(cpuRequest string, memoryRequest string, cpuLimit string, memoryLimit string) v1.ResourceRequirements {
	resource := v1.ResourceRequirements{
		Requests: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(cpuRequest),
			v1.ResourceMemory: resource.MustParse(memoryRequest),
		},
		Limits: v1.ResourceList{
			v1.ResourceCPU:    resource.MustParse(cpuLimit),
			v1.ResourceMemory: resource.MustParse(memoryLimit),
		},
	}
	return resource
}

func (kc *KubernetesConfig) GetRemainingNodeResources() (map[string]NodeResources, error) {
	cfg, err := GetVendorConfig()
	if err != nil {
		fmt.Printf("[ERROR] Failed to get vendor config: %v\n", err)
		return nil, err
	}

	ctx := context.TODO()
	nodes, err := kc.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("[ERROR] Failed to list nodes: %v\n", err)
		return nil, err
	}

	nodeResources := make(map[string]NodeResources)
	for i, node := range nodes.Items {
		pods, err := kc.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			FieldSelector: cfg.FieldSelectorPrefix + node.Name,
		})
		if err != nil {
			return nil, err
		}

		instanceType := node.Labels[cfg.InstanceTypeLabel]
		capacityType := node.Labels[cfg.CapacityTypeLabel]
		nodeGroup := node.Labels[cfg.NodeGroupLabel]
		engine := node.Labels[cfg.NodeSelectorPrefix]
		ip := node.Name
		usedCPU := resource.Quantity{}
		usedMemory := resource.Quantity{}
		usedGPU := resource.Quantity{}
		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				usedCPU.Add(container.Resources.Requests[v1.ResourceCPU])
				usedMemory.Add(container.Resources.Requests[v1.ResourceMemory])
				if val, ok := container.Resources.Requests[v1.ResourceName(cfg.GPUVendorLabel)]; ok {
					usedGPU.Add(val)
				}
			}
		}

		totalCPU := node.Status.Allocatable[v1.ResourceCPU]
		totalMemory := node.Status.Allocatable[v1.ResourceMemory]
		totalGPU := node.Status.Allocatable[v1.ResourceName(cfg.GPUVendorLabel)]

		// Calculate remaining
		remainingCPU := totalCPU.DeepCopy()
		remainingCPU.Sub(usedCPU)
		cpuCoresRemaining := remainingCPU.MilliValue() / 1000

		remainingMemory := totalMemory.DeepCopy()
		remainingMemory.Sub(usedMemory)
		memGiBRemaining := int(remainingMemory.Value() / (1024 * 1024 * 1024))

		remainingGPU := totalGPU.DeepCopy()
		remainingGPU.Sub(usedGPU)
		gpuRemaining := remainingGPU.Value()

		nodeName := fmt.Sprintf("node_%d", i+1)
		nodeResources[nodeName] = NodeResources{
			CPU:          fmt.Sprintf("%d", cpuCoresRemaining),
			Memory:       fmt.Sprintf("%d", memGiBRemaining),
			GPU:          fmt.Sprintf("%d", gpuRemaining),
			InstanceType: instanceType,
			CapacityType: capacityType,
			NodeGroup:    nodeGroup,
			Type:         engine,
			IP:           ip,
		}
	}

	return nodeResources, nil
}

func (kc *KubernetesConfig) GetNodeTotalResources() (map[string]NodeResources, error) {
	cfg, err := GetVendorConfig()
	if err != nil {
		return nil, err
	}

	ctx := context.TODO()
	nodes, err := kc.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	nodeResources := make(map[string]NodeResources)
	for i, node := range nodes.Items {
		instanceType := node.Labels[cfg.InstanceTypeLabel]
		capacityType := node.Labels[cfg.CapacityTypeLabel]
		nodeGroup := node.Labels[cfg.NodeGroupLabel]
		engine := node.Labels[cfg.NodeSelectorPrefix]
		ip := node.Name

		totalCPU := node.Status.Allocatable[v1.ResourceCPU]
		cpuCores := totalCPU.MilliValue() / 1000

		totalMemory := node.Status.Allocatable[v1.ResourceMemory]
		memGiB := totalMemory.Value() / (1024 * 1024 * 1024)

		totalGPU := node.Status.Allocatable[v1.ResourceName(cfg.GPUVendorLabel)]
		gpuInt := totalGPU.Value()

		nodeName := fmt.Sprintf("node_%d", i+1)
		nodeResources[nodeName] = NodeResources{
			CPU:          fmt.Sprintf("%d", cpuCores),
			Memory:       fmt.Sprintf("%d", memGiB),
			GPU:          fmt.Sprintf("%d", gpuInt),
			InstanceType: instanceType,
			CapacityType: capacityType,
			NodeGroup:    nodeGroup,
			Type:         engine,
			IP:           ip,
		}
	}

	return nodeResources, nil
}

func (kc *KubernetesConfig) CheckCpuAvailability(numOfCpu string) (bool, error) {
	nodeResources, err := kc.GetRemainingNodeResources()
	if err != nil {
		return false, err
	}

	numOfCpuFloat, _ := strconv.ParseFloat(numOfCpu, 64)
	for _, resources := range nodeResources {
		remainingCpuFloat, _ := strconv.ParseFloat(resources.CPU, 64)
		if remainingCpuFloat >= numOfCpuFloat {
			return true, nil
		}
	}

	return false, fmt.Errorf("the requested CPU resources are not available")
}

func (kc *KubernetesConfig) CheckMemoryAvailability(memoryRequest string) (bool, error) {
	nodeResources, err := kc.GetRemainingNodeResources()
	if err != nil {
		return false, err
	}
	memoryRequestQuantity := resource.MustParse(memoryRequest)
	memoryRequestBytes := memoryRequestQuantity.Value()

	for _, resources := range nodeResources {
		remainingMemoryQuantity := resource.MustParse(resources.Memory)
		remainingMemoryBytes := GigabytesToBytes(remainingMemoryQuantity.Value())
		fmt.Println(memoryRequestBytes, remainingMemoryBytes)
		if remainingMemoryBytes > memoryRequestBytes {
			return true, nil
		}
	}
	return false, fmt.Errorf("the requested memory resources are not available")
}

func (kc *KubernetesConfig) CheckGpuAvailability(numOfGpu string) (bool, error) {
	nodeResources, err := kc.GetRemainingNodeResources()
	if err != nil {
		return false, err
	}
	numOfGpuFloat, _ := strconv.ParseFloat(numOfGpu, 64)
	for _, resources := range nodeResources {
		remainingGpuFloat, _ := strconv.ParseFloat(resources.GPU, 64)
		if remainingGpuFloat >= numOfGpuFloat {
			return true, nil
		}
	}

	return false, fmt.Errorf("the requested GPU resources are not available")
}
