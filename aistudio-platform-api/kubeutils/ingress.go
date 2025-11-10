package kubeutils

import (
	"context"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (kc *KubernetesConfig) AppendRuleToIngress(namespace, ingressName, serviceName, path string) error {

	ingressClient := kc.Clientset.NetworkingV1().Ingresses(namespace)

	ingress, err := ingressClient.Get(context.TODO(), ingressName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	pathType := networkingv1.PathTypePrefix
	newPath := networkingv1.HTTPIngressPath{
		Path:     path,
		PathType: &pathType,
		Backend: networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: serviceName,
				Port: networkingv1.ServiceBackendPort{
					Number: 80,
				},
			},
		},
	}

	for i := range ingress.Spec.Rules {
		if ingress.Spec.Rules[i].HTTP != nil {
			for _, path := range ingress.Spec.Rules[i].HTTP.Paths {
				if path.Path == newPath.Path {
					return nil
				}
			}
			ingress.Spec.Rules[i].HTTP.Paths = append(ingress.Spec.Rules[i].HTTP.Paths, newPath)
		}
	}

	_, err = ingressClient.Update(context.TODO(), ingress, metav1.UpdateOptions{})
	return err
}

func (kc *KubernetesConfig) DeleteRuleFromIngress(namespace, path, ingressName string) error {

	ingressClient := kc.Clientset.NetworkingV1().Ingresses(namespace)

	ingress, err := ingressClient.Get(context.TODO(), ingressName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for j, candidate := range rule.HTTP.Paths {
			if candidate.Path == path {
				rule.HTTP.Paths = append(rule.HTTP.Paths[:j], rule.HTTP.Paths[j+1:]...)
				break
			}
		}
	}

	_, err = ingressClient.Update(context.TODO(), ingress, metav1.UpdateOptions{})
	return err
}
