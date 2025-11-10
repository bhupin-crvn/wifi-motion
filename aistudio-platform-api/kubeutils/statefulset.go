package kubeutils

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2/log"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateContainerConfig(containerName string, image string, containerPort int, volumeMounts []apiv1.VolumeMount, envVars []apiv1.EnvVar) apiv1.Container {

	return apiv1.Container{
		Name:            containerName,
		Image:           image,
		VolumeMounts:    volumeMounts,
		Env:             envVars,
		ImagePullPolicy: apiv1.PullIfNotPresent,
		Ports: []apiv1.ContainerPort{
			{
				ContainerPort: int32(containerPort),
			},
		},
	}
}

func (kc *KubernetesConfig) ConfigStatefulSet(newNamespace string, name string, serviceName string, gpuRequest int, notebookPort int, diskStorage string, nodeSelector string, resources apiv1.ResourceRequirements, containers []apiv1.Container, volumes []apiv1.Volume) {
	storageClassName := "nfs-csi-model"
	statefulsetsClient := kc.Clientset.AppsV1().StatefulSets(newNamespace)

	// Apply resources to each container
	for i := range containers {
		containers[i].Resources = resources
	}

	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: serviceName,
			Replicas:    int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: apiv1.PodSpec{
					NodeSelector: map[string]string{
						"type": nodeSelector,
					},
					SecurityContext: &apiv1.PodSecurityContext{
						RunAsUser:  int64Ptr(0),
						RunAsGroup: int64Ptr(0),
						FSGroup:    int64Ptr(1000),
					},
					Volumes:    volumes,
					Containers: containers,
				},
			},
			VolumeClaimTemplates: []apiv1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "jl",
					},
					Spec: apiv1.PersistentVolumeClaimSpec{
						AccessModes: []apiv1.PersistentVolumeAccessMode{
							apiv1.ReadWriteMany,
						},
						StorageClassName: &storageClassName,
						Resources: apiv1.ResourceRequirements{
							Requests: apiv1.ResourceList{
								apiv1.ResourceStorage: resource.MustParse(diskStorage),
							},
						},
					},
				},
			},
		},
	}

	if gpuRequest > 0 {
		configGpu(&statefulset.Spec.Template.Spec, strconv.Itoa(gpuRequest))
	}

	log.Info("Creating statefulset...")
	result, err := statefulsetsClient.Create(context.TODO(), statefulset, metav1.CreateOptions{})
	if err != nil {
		log.Error("Error in creating labspace: ", err.Error())
		return
	}
	log.Info("Created statefulset %q.\n", result.GetObjectMeta().GetName())
}

func (kc *KubernetesConfig) CreateStatefulSet(newNamespace string, name string, serviceName string, image string, gpuRequest int, notebookPort int, diskStorage string, nodeSelector string, resources apiv1.ResourceRequirements, envVars []apiv1.EnvVar) {
	volumes, volumeMounts := kc.CreateVolumesAndMounts(gpuRequest)
	container := CreateContainerConfig(name, image, notebookPort, volumeMounts, envVars)
	kc.ConfigStatefulSet(newNamespace, name, serviceName, gpuRequest, notebookPort, diskStorage, nodeSelector, resources, []apiv1.Container{container}, volumes)
}

func (kc *KubernetesConfig) CreateStatefulSetWithDualContainer(newNamespace, name, serviceName, image, imageAdk string, gpuRequest, notebookPort, adkPort int, diskStorage, nodeSelector string, resources apiv1.ResourceRequirements, env [][]apiv1.EnvVar) {
	volumes, volumeMounts := kc.CreateVolumesAndMounts(gpuRequest)
	env1 := []apiv1.EnvVar{}
	env2 := []apiv1.EnvVar{}
	if len(env) > 0 {
		env1 = env[0]
	}
	if len(env) > 1 {
		env2 = env[1]
	}
	container1 := CreateContainerConfig(name, image, notebookPort, volumeMounts, env1)
	container2 := CreateContainerConfig("adk", imageAdk, adkPort, volumeMounts, env2)
	containers := []apiv1.Container{container1, container2}
	kc.ConfigStatefulSet(newNamespace, name, serviceName, gpuRequest, notebookPort, diskStorage, nodeSelector, resources, containers, volumes)
}

func (kc *KubernetesConfig) DeleteStatefulSet(namespace string, statefulSetName string) {
	statefulSetClient := kc.Clientset.AppsV1().StatefulSets(namespace)
	deletePolicy := metav1.DeletePropagationForeground
	if err := statefulSetClient.Delete(context.TODO(), statefulSetName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		log.Error("Error in deleting labspace: ", err.Error())
		return
	}
	log.Info("Deleted StatefulSet %s in namespace %s\n", statefulSetName, namespace)
}