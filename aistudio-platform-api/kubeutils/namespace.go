package kubeutils

import (
	"context"
	"fmt"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


func (kc *KubernetesConfig) CreateNamespace(namespace string) string {
	ns := &apiv1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	ns, err := kc.Clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if errors.IsAlreadyExists(err) {
		fmt.Printf("Namespace %s already exists\n", namespace)
	} else if err != nil {
		panic(err.Error())
	}
	return fmt.Sprintf("%+v\n", ns.Status)
}
