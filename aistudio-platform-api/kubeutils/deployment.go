package kubeutils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/gofiber/fiber/v2/log"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (kc *KubernetesConfig) ConfigModelDeployment(newNamespace string, deploymentName string, Image string, pvcName string, gpuRequest int, modelPort int, nodeSelector string, resources apiv1.ResourceRequirements, envVars []apiv1.EnvVar) {

	deploymentsClient := kc.Clientset.AppsV1().Deployments(newNamespace)
	VolumeMounts := []apiv1.VolumeMount{
		{
			Name:      pvcName,
			MountPath: "/deploy/deployment",
		},
		{
			Name:      "xtract",
			MountPath: "/deploy/be_ml_data",
		},
	}
	container := CreateContainerConfig(deploymentName, Image, modelPort, VolumeMounts, envVars)
	container.Resources = resources
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploymentName,
			Labels: map[string]string{
				"app": deploymentName,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": deploymentName,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": deploymentName,
					},
				},
				Spec: apiv1.PodSpec{
					NodeSelector: map[string]string{
						"type": nodeSelector,
					},
					Volumes: []apiv1.Volume{
						{
							Name: pvcName,
							VolumeSource: apiv1.VolumeSource{
								PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
						{
							Name: "xtract",
							VolumeSource: apiv1.VolumeSource{
								PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
									ClaimName: "xtract",
								},
							},
						},
					},
					Containers: []apiv1.Container{
						container,
					},
				},
			},
		},
	}
	{
		if gpuRequest > 0 {
			configGpu(&deployment.Spec.Template.Spec, strconv.Itoa(gpuRequest))
		}

		fmt.Println("Creating deployment...")
		result, err := deploymentsClient.Create(context.TODO(), deployment, metav1.CreateOptions{})
		if err != nil {
			log.Error(err.Error(), "Error in creatinng deployment: ", deploymentName)
		}
		fmt.Printf("Created deployment %q.\n", result.GetObjectMeta().GetName())
	}
}


func (kc *KubernetesConfig) ConfigDeployment(newNamespace, deploymentName, image string, port int, nodeSelector string, envVars []apiv1.EnvVar) (string, error) {
    deploymentsClient := kc.Clientset.AppsV1().Deployments(newNamespace)

    container := apiv1.Container{
        Name:  deploymentName,
        Image: image,
        Ports: []apiv1.ContainerPort{
            {
                ContainerPort: int32(port),
            },
        },
        Env:       envVars,
    }

    deployment := &appsv1.Deployment{
        ObjectMeta: metav1.ObjectMeta{
            Name:      deploymentName,
            Namespace: newNamespace,
            Labels: map[string]string{
                "app": deploymentName,
            },
        },
        Spec: appsv1.DeploymentSpec{
            Replicas: int32Ptr(1),
            Selector: &metav1.LabelSelector{
                MatchLabels: map[string]string{
                    "app": deploymentName,
                },
            },
            Template: apiv1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{
                    Labels: map[string]string{
                        "app": deploymentName,
                    },
                },
                Spec: apiv1.PodSpec{
                    NodeSelector: map[string]string{
                        "type": nodeSelector,
                    },
                    Containers: []apiv1.Container{
                        container,
                    },
                },
            },
        },
    }

    fmt.Printf("Creating deployment %q in namespace %q...\n", deploymentName, newNamespace)
    result, err := deploymentsClient.Create(context.TODO(), deployment, metav1.CreateOptions{})
    if err != nil {
        return "", fmt.Errorf("failed to create deployment %s: %w", deploymentName, err)
    }

    deploymentNameResult := result.GetObjectMeta().GetName()
    fmt.Printf("Created deployment %q.\n", deploymentNameResult)
    return deploymentNameResult, nil
}

func (kc *KubernetesConfig) DeleteDeployment(namespace string, deploymentName string) {

	deploymentsClient := kc.Clientset.AppsV1().Deployments(namespace)
	fmt.Println("Deleting deployment...")
	deletePolicy := metav1.DeletePropagationForeground
	if err := deploymentsClient.Delete(context.TODO(), deploymentName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		log.Error(err.Error(), "Error while deleting the pod")
	}

	fmt.Printf("Deleted deployment %q.\n", deploymentName)
}

func (kc *KubernetesConfig) ModelDeploymentExists(namespace string, deploymentName string) bool {

	deploymentsClient := kc.Clientset.AppsV1().Deployments(namespace)

	_, err := deploymentsClient.Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}
		log.Fatalf("Failed to get deployments: %v", err)
	}

	return true
}

func (kc *KubernetesConfig) GetDeploymentLogs(deploymentName string, namespace string) (string, error) {
	pods, err := kc.Clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + deploymentName,
	})
	if err != nil {
		return "", fmt.Errorf("error getting pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for deployment: %s", deploymentName)
	}
	podName := pods.Items[0].Name

	req := kc.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &apiv1.PodLogOptions{})
	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("error in opening stream: %w", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("error in copy information from podLogs to buffer: %w", err)
	}

	str := buf.String()

	return str, nil
}
