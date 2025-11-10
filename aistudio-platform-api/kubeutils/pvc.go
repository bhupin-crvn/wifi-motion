package kubeutils

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2/log"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (kc *KubernetesConfig) CreateVolumesAndMounts(gpuRequest int) ([]apiv1.Volume, []apiv1.VolumeMount) {
	storageStr := "40"
	storage, _ := strconv.Atoi(storageStr)
	volumes := []apiv1.Volume{
		{
			Name: "aim-runs",
			VolumeSource: apiv1.VolumeSource{
				PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
					ClaimName: "aim-runs-claim",
				},
			},
		},
	}

	volumeMounts := []apiv1.VolumeMount{
		{
			Name:      "jl",
			MountPath: "/home/studio/work",
		},
		{
			Name:      "aim-runs",
			MountPath: "/aim",
		},
	}
	totalStorage := storage * 1024 * 1024 * 1024
	if gpuRequest > 0 {
		sizeLimit := resource.NewQuantity(int64(totalStorage), resource.BinarySI)
		volumes = append(volumes, apiv1.Volume{
			Name: "dshm",
			VolumeSource: apiv1.VolumeSource{
				EmptyDir: &apiv1.EmptyDirVolumeSource{
					Medium:    apiv1.StorageMediumMemory,
					SizeLimit: sizeLimit,
				},
			},
		})

		volumeMounts = append(volumeMounts, apiv1.VolumeMount{
			Name:      "dshm",
			MountPath: "/dev/shm",
		})
	}

	return volumes, volumeMounts
}

func (kc *KubernetesConfig) CreatePersistentVolume(newNamespace string, pvcName string, diskStorage string) error {
	storageClassName := "nfs-csi-model"
	pvcClient := kc.Clientset.CoreV1().PersistentVolumeClaims(newNamespace)
	pvc := &apiv1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: pvcName,
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
	}
	_, err := pvcClient.Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PersistentVolume: %w", err)
	}
	return nil
}

func (kc *KubernetesConfig) DeletePersistentVolume(namespace string, pvcName string) error {
	pvcClient := kc.Clientset.CoreV1().PersistentVolumeClaims(namespace)
	err := pvcClient.Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete PersistentVolume: %w", err)
	}
	return nil
}

func (kc *KubernetesConfig) PersistentVolumeExists(namespace string, pvcName string) bool {
	_, err := kc.Clientset.CoreV1().PersistentVolumes().Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}

		log.Error(err.Error(), "Error in persistent volume")
	}

	return true
}
