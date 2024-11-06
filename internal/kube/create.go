package kube

import (
	"context"
	"fmt"

	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateResource creates a single resource by resource type, name, namespace and resource.
func (c Cluster) CreateResource(resourceType ResourceType, name, namespace string, resource interface{}) error {
	switch resourceType {
	case Deployment:
		_, err := c.Clientset.AppsV1().Deployments(namespace).Create(context.TODO(), resource.(*appsv1.Deployment), metav1.CreateOptions{})
		return err
	case ConfigMap:
		_, err := c.Clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), resource.(*corev1.ConfigMap), metav1.CreateOptions{})
		return err
	case Ingress:
		_, err := c.Clientset.NetworkingV1().Ingresses(namespace).Create(context.TODO(), resource.(*networkingv1.Ingress), metav1.CreateOptions{})
		return err
	case Secret:
		_, err := c.Clientset.CoreV1().Secrets(namespace).Create(context.TODO(), resource.(*corev1.Secret), metav1.CreateOptions{})
		return err
	case Namespace:
		_, err := c.Clientset.CoreV1().Namespaces().Create(context.TODO(), resource.(*corev1.Namespace), metav1.CreateOptions{})
		return err
	case Service:
		_, err := c.Clientset.CoreV1().Services(namespace).Create(context.TODO(), resource.(*corev1.Service), metav1.CreateOptions{})
		return err
	case Middleware:
		_, err := c.TraefikClientset.TraefikV1alpha1().Middlewares(namespace).Create(context.TODO(), resource.(*traefikv1alpha1.Middleware), metav1.CreateOptions{})
		return err
	case IngressRoute:
		_, err := c.TraefikClientset.TraefikV1alpha1().IngressRoutes(namespace).Create(context.TODO(), resource.(*traefikv1alpha1.IngressRoute), metav1.CreateOptions{})
		return err
	case IngressRouteTCP:
		_, err := c.TraefikClientset.TraefikV1alpha1().IngressRouteTCPs(namespace).Create(context.TODO(), resource.(*traefikv1alpha1.IngressRouteTCP), metav1.CreateOptions{})
		return err
	case IngressRouteUDP:
		_, err := c.TraefikClientset.TraefikV1alpha1().IngressRouteUDPs(namespace).Create(context.TODO(), resource.(*traefikv1alpha1.IngressRouteUDP), metav1.CreateOptions{})
		return err
	case TraefikService:
		_, err := c.TraefikClientset.TraefikV1alpha1().TraefikServices(namespace).Create(context.TODO(), resource.(*traefikv1alpha1.TraefikService), metav1.CreateOptions{})
		return err
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}
