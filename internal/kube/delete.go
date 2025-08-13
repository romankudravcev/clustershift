package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c Cluster) DeleteResource(resourceType ResourceType, name, namespace string) error {
	switch resourceType {
	case Deployment:
		return c.Clientset.AppsV1().Deployments(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case ConfigMap:
		return c.Clientset.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case Ingress:
		return c.Clientset.NetworkingV1().Ingresses(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case Secret:
		return c.Clientset.CoreV1().Secrets(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case Namespace:
		return c.Clientset.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{})
	case Service:
		return c.Clientset.CoreV1().Services(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case ServiceAccount:
		return c.Clientset.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case ClusterRole:
		return c.Clientset.RbacV1().ClusterRoles().Delete(context.TODO(), name, metav1.DeleteOptions{})
	case ClusterRoleBind:
		return c.Clientset.RbacV1().ClusterRoleBindings().Delete(context.TODO(), name, metav1.DeleteOptions{})
	case StatefulSet:
		return c.Clientset.AppsV1().StatefulSets(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case Pod:
		return c.Clientset.CoreV1().Pods(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case Middleware:
		return c.TraefikClientset.TraefikV1alpha1().Middlewares(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case IngressRoute:
		return c.TraefikClientset.TraefikV1alpha1().IngressRoutes(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case IngressRouteTCP:
		return c.TraefikClientset.TraefikV1alpha1().IngressRouteTCPs(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case IngressRouteUDP:
		return c.TraefikClientset.TraefikV1alpha1().IngressRouteUDPs(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	case TraefikService:
		return c.TraefikClientset.TraefikV1alpha1().TraefikServices(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}
