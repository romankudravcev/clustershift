package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// FetchResources fetches all resources by resource type.
func (c Cluster) FetchResources(resourceType ResourceType) (interface{}, error) {
	switch resourceType {
	case Deployment:
		return c.Clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
	case ConfigMap:
		return c.Clientset.CoreV1().ConfigMaps("").List(context.TODO(), metav1.ListOptions{})
	case Ingress:
		return c.Clientset.NetworkingV1().Ingresses("").List(context.TODO(), metav1.ListOptions{})
	case Secret:
		return c.Clientset.CoreV1().Secrets("").List(context.TODO(), metav1.ListOptions{})
	case Namespace:
		return c.Clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	case Service:
		return c.Clientset.CoreV1().Services("").List(context.TODO(), metav1.ListOptions{})
	case ServiceAccount:
		return c.Clientset.CoreV1().ServiceAccounts("").List(context.TODO(), metav1.ListOptions{})
	case ClusterRole:
		return c.Clientset.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{})
	case ClusterRoleBind:
		return c.Clientset.RbacV1().ClusterRoleBindings().List(context.TODO(), metav1.ListOptions{})
	case Middleware:
		return c.TraefikClientset.TraefikV1alpha1().Middlewares("").List(context.TODO(), metav1.ListOptions{})
	case IngressRoute:
		return c.TraefikClientset.TraefikV1alpha1().IngressRoutes("").List(context.TODO(), metav1.ListOptions{})
	case IngressRouteTCP:
		return c.TraefikClientset.TraefikV1alpha1().IngressRouteTCPs("").List(context.TODO(), metav1.ListOptions{})
	case IngressRouteUDP:
		return c.TraefikClientset.TraefikV1alpha1().IngressRouteUDPs("").List(context.TODO(), metav1.ListOptions{})
	case TraefikService:
		return c.TraefikClientset.TraefikV1alpha1().TraefikServices("").List(context.TODO(), metav1.ListOptions{})
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// FetchResource fetches a single resource by resource type, name, and namespace.
func (c Cluster) FetchResource(resourceType ResourceType, name string, namespace string) (interface{}, error) {
	switch resourceType {
	case Deployment:
		return c.Clientset.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case ConfigMap:
		return c.Clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case Ingress:
		return c.Clientset.NetworkingV1().Ingresses(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case Secret:
		return c.Clientset.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case Namespace:
		return c.Clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	case Service:
		return c.Clientset.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case ServiceAccount:
		return c.Clientset.CoreV1().ServiceAccounts(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case ClusterRole:
		return c.Clientset.RbacV1().ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
	case ClusterRoleBind:
		return c.Clientset.RbacV1().ClusterRoleBindings().Get(context.TODO(), name, metav1.GetOptions{})
	case Middleware:
		return c.TraefikClientset.TraefikV1alpha1().Middlewares(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case IngressRoute:
		return c.TraefikClientset.TraefikV1alpha1().IngressRoutes(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case IngressRouteTCP:
		return c.TraefikClientset.TraefikV1alpha1().IngressRouteTCPs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case IngressRouteUDP:
		return c.TraefikClientset.TraefikV1alpha1().IngressRouteUDPs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	case TraefikService:
		return c.TraefikClientset.TraefikV1alpha1().TraefikServices(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

// FetchCustomResources fetches all custom resources of a specific type across all namespaces
func (c Cluster) FetchCustomResources(group, version, resource string) ([]map[string]interface{}, error) {
	// Define the GVR (GroupVersionResource)
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// List the custom resources
	list, err := c.DynamicClientset.Resource(gvr).
		Namespace(""). // empty namespace means all namespaces
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Convert to a slice of maps for generic handling
	var resources []map[string]interface{}
	for _, item := range list.Items {
		resources = append(resources, item.Object)
	}

	return resources, nil
}

// FetchCustomResource fetches a single custom resource by name and namespace
func (c Cluster) FetchCustomResource(group, version, resource, namespace, name string) (map[string]interface{}, error) {
	// Define the GVR (GroupVersionResource)
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// Get the specific custom resource
	obj, err := c.DynamicClientset.Resource(gvr).
		Namespace(namespace).
		Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return obj.Object, nil
}
