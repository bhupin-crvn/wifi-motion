package kubeutils

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v2/log"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (kc *KubernetesConfig) CreateService(newNamespace string, serviceName string, lable string, port int,serviceType apiv1.ServiceType) {

	servicesClient := kc.Clientset.CoreV1().Services(newNamespace)
	service := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Labels: map[string]string{
				"app": lable,
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: serviceType,
			Ports: []apiv1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(port),
					NodePort:   0,
				},
			},
			Selector: map[string]string{
				"app": lable,
			},
		},
	}
	fmt.Println("Creating service...")
	resultSvc, err := servicesClient.Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		log.Error(err.Error(), "Error while creating service", serviceName)
	}
	fmt.Printf("[SERVICE-CREATED] %q.\n", resultSvc.GetObjectMeta().GetName())
	
}

func (kc *KubernetesConfig) DeleteService(namespace string, serviceName string) {

	servicesClient := kc.Clientset.CoreV1().Services(namespace)
	deletePolicy := metav1.DeletePropagationForeground

	if err := servicesClient.Delete(context.TODO(), serviceName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}); err != nil {
		log.Error(err.Error(), "Error while Deleteing service", serviceName)
	}
	fmt.Println("Deleted services.")
}

func (kc *KubernetesConfig) ServiceExists(namespace string, serviceName string) bool {
	_, err := kc.Clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false
		}

		log.Error(err.Error(), serviceName, "Doesnot exist in the system")
	}

	return true
}
