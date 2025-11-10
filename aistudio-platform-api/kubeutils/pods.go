package kubeutils

import (
	"context"
	"fmt"
	"strings"
	"time"

	// "github.com/gofiber/fiber/v2/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (kc *KubernetesConfig) ListPods(namespace string) ([]map[string]string, error) {
	podlist := kc.Clientset.CoreV1().Pods(namespace)
	pods, err := podlist.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		err = fmt.Errorf("error getting pods: %v ", err)
		return nil, err
	}
	var data []map[string]string

	if len(pods.Items) == 0 {
		return []map[string]string{}, nil
	}
	processedDeployments := make(map[string]bool)
	deploymentPods := make(map[string][]map[string]string)

	for _, pod := range pods.Items {

		deploymentName := ""
		if pod.Labels != nil {
			deploymentName = pod.Labels["app"]
		}
		if deploymentName == "" {
			deploymentName = pod.GetName()
		}

		podCreationTime := pod.GetCreationTimestamp()
		age := time.Since(podCreationTime.Time).Round(time.Second)

		// Get the status of each of the pods
		podStatus := pod.Status

		var containerRestarts int32
		var containerReady int
		var totalContainers int
		var ready string
		var status string

		// --- ENHANCED STATUS LOGIC START ---
		if pod.DeletionTimestamp != nil {
			status = "Terminating"
		} else {
			// Look at all container statuses to find the most specific reason
			if len(podStatus.ContainerStatuses) > 0 {
				var worstReason string
				var worstPriority int

				for _, cs := range podStatus.ContainerStatuses {
					// Handle Waiting state
					if cs.State.Waiting != nil {
						reason := cs.State.Waiting.Reason
						message := strings.ToLower(cs.State.Waiting.Message)

						prio := 0
						switch reason {
						case "ErrImagePull", "ImagePullBackOff":
							prio = 6
						case "CrashLoopBackOff":
							prio = 5
						case "CreateContainerConfigError", "CreateContainerError":
							prio = 4
						case "ContainerCreating":
							prio = 3
							// Detect migration/init-related activity
							if strings.Contains(message, "migration") ||
								strings.Contains(message, "migrate") ||
								strings.Contains(message, "init") ||
								strings.Contains(message, "flyway") ||
								strings.Contains(message, "liquibase") {
								reason = "Migrating"
								prio = 7
							}
						default:
							prio = 1
						}

						if prio > worstPriority {
							worstPriority = prio
							worstReason = reason
							if cs.State.Waiting.Message != "" && reason != "Migrating" {
								worstReason = fmt.Sprintf("%s (%s)", reason, cs.State.Waiting.Message)
							}
						}
					}

					// Handle Terminated state (highest priority except Terminating)
					if cs.State.Terminated != nil {
						var term string
						if cs.State.Terminated.Reason != "" {
							term = cs.State.Terminated.Reason
						} else if cs.State.Terminated.Signal != 0 {
							term = fmt.Sprintf("Signal:%d", cs.State.Terminated.Signal)
						} else {
							term = fmt.Sprintf("ExitCode:%d", cs.State.Terminated.ExitCode)
						}
						worstReason = term
						worstPriority = 10
					}
				}

				if worstReason != "" {
					status = worstReason
				}
			}

			// Fallback to pod phase
			if status == "" {
				if podStatus.Phase == "PodInitializing" {
					status = "Initializing"
				} else {
					status = string(podStatus.Phase)
				}
			}
		}

		// If still empty (shouldn't happen), use phase
		if status == "" {
			status = string(podStatus.Phase)
		}
		// --- ENHANCED STATUS LOGIC END ---

		// Count containers and readiness
		totalContainers = len(pod.Spec.Containers)
		if len(podStatus.ContainerStatuses) > 0 {
			for _, cs := range podStatus.ContainerStatuses {
				containerRestarts += cs.RestartCount
				if cs.Ready {
					containerReady++
				}
			}
		}

		name := pod.GetName()
		ready = fmt.Sprintf("%d/%d", containerReady, totalContainers)
		restarts := fmt.Sprintf("%d", containerRestarts)
		ageS := age.String()

		podInfo := map[string]string{
			"name":     name,
			"ready":    ready,
			"status":   status,
			"restarts": restarts,
			"age":      ageS,
		}
		deploymentPods[deploymentName] = append(deploymentPods[deploymentName], podInfo)
	}

	// Select one representative pod per deployment (prefer Running)
	for deploymentName, pods := range deploymentPods {
		if processedDeployments[deploymentName] {
			continue
		}

		var selectedPod map[string]string
		for _, pod := range pods {
			if pod["status"] == "Running" {
				selectedPod = pod
				break
			}
		}
		if selectedPod == nil && len(pods) > 0 {
			selectedPod = pods[0]
		}

		if selectedPod != nil {
			data = append(data, selectedPod)
			processedDeployments[deploymentName] = true
		}
	}
	return data, nil
}
func (kc *KubernetesConfig) GetPodDetail(id string, namespace string) (map[string]string, error) {
	podlist := kc.Clientset.CoreV1().Pods(namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: "app=" + id,
	}
	pods, err := podlist.List(context.Background(), listOptions)
	if err != nil {
		err = fmt.Errorf("error getting pods: %v ", err)
		return nil, err
	}
	var data map[string]string
	for _, pod := range pods.Items {
		podCreationTime := pod.GetCreationTimestamp()
		age := time.Since(podCreationTime.Time).Round(time.Second)

		// Get the status of each of the pods
		podStatus := pod.Status

		var containerRestarts int32
		var containerReady int
		var totalContainers int
		var ready string
		//If a pod has multiple containers, get the status from all
		if len(pod.Spec.Containers) > 0 {
			for container := range pod.Spec.Containers {
				containerRestarts += podStatus.ContainerStatuses[container].RestartCount
				if podStatus.ContainerStatuses[container].Ready {
					containerReady++
				}
				totalContainers++
			}
		}

		// Get the values from the pod status
		name := pod.GetName()
		ready = fmt.Sprintf("%v/%v", containerReady, totalContainers)
		status := fmt.Sprintf("%v", podStatus.Phase)
		restarts := fmt.Sprintf("%v", containerRestarts)
		ageS := age.String()

		// Append this to data to be printed in a table
		data = map[string]string{"name": name, "ready": ready, "status": status, "restarts": restarts, "age": ageS}
	}
	// log.Info("Request the detail of pod: ", id)
	return data, nil
}

func (kc *KubernetesConfig) GetDeploymentPodEvents(deploymentName, podNamespace string) ([]map[string]string, error) {
	deploymentsClient := kc.Clientset.AppsV1().Deployments(podNamespace)
	ctx := context.TODO()

	// Get the deployment to find out which pods belong to it
	deployment, err := deploymentsClient.Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	// Get pods belonging to the deployment
	pods, err := kc.Clientset.CoreV1().Pods(podNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(deployment.Spec.Selector.MatchLabels).String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var eventDetails []map[string]string

	// Iterate over each pod and fetch events
	for _, pod := range pods.Items {
		events, err := kc.Clientset.CoreV1().Events(podNamespace).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s", pod.Name),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list events for pod %s: %w", pod.Name, err)
		}

		for _, event := range events.Items {
			eventDetail := map[string]string{
				"PodName": pod.Name,
				"Type":    event.Type,
				"Reason":  event.Reason,
				"Message": event.Message,
			}
			eventDetails = append(eventDetails, eventDetail)
		}
	}

	return eventDetails, nil
}
