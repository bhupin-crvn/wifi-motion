package kubeutils

import (
	"context"
	"fmt"
	"log"
	"math"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetPodMetrics(namespace string) {
	config := NewKubernetesConfig()
	metricsClient := config.MetricsClient
	podMetricsList, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error getting pod metrics: %v", err)
	}

	for _, podMetrics := range podMetricsList.Items {
		fmt.Printf("Pod: %s\n", podMetrics.Name)
		for _, container := range podMetrics.Containers {
			fmt.Printf("  Container: %s\n", container.Name)
			fmt.Printf("    CPU Usage: %s\n", container.Usage.Cpu().String())
			fmt.Printf("    Memory Usage: %s\n", container.Usage.Memory().String())
		}
	}
}

func (kc *KubernetesConfig) GetPodMetric(namespace string) ([]PodMetrics, error) {
	metricsClient := kc.MetricsClient
	podMetricsList, err := metricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting pod metrics: %v", err)
	}

	var metrics []PodMetrics
	for _, podMetrics := range podMetricsList.Items {
		for _, container := range podMetrics.Containers {
		
			gpuUsage := "0"
			if gpu, ok := container.Usage["nvidia.com/gpu"]; ok {
				gpuUsage = gpu.String()
			}

			cpuQuantity := container.Usage.Cpu().MilliValue()
			cpuCores := int(math.Ceil(float64(cpuQuantity) / 1000))

			memoryQuantity := container.Usage.Memory()
			memoryKB := int(memoryQuantity.Value() / 1024)

			metrics = append(metrics, PodMetrics{
				PodName:     podMetrics.Name,
				CPUUsage:    fmt.Sprintf("%d", cpuCores),
				GPUUsage:    gpuUsage,
				MemoryUsage: fmt.Sprintf("%d KB", memoryKB),
			})
		}
	}
	return metrics, nil
}
