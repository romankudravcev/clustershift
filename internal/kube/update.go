package kube

import (
	"context"
	"fmt"
	"strings"

	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/restmapper"
)

func (c Cluster) UpdateResource(resourceType ResourceType, name, namespace string, resource interface{}) error {
	switch resourceType {
	case Deployment:
		_, err := c.Clientset.AppsV1().Deployments(namespace).Update(context.TODO(), resource.(*appsv1.Deployment), metav1.UpdateOptions{})
		return err
	case ConfigMap:
		_, err := c.Clientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), resource.(*corev1.ConfigMap), metav1.UpdateOptions{})
		return err
	case Ingress:
		_, err := c.Clientset.NetworkingV1().Ingresses(namespace).Update(context.TODO(), resource.(*networkingv1.Ingress), metav1.UpdateOptions{})
		return err
	case Secret:
		_, err := c.Clientset.CoreV1().Secrets(namespace).Update(context.TODO(), resource.(*corev1.Secret), metav1.UpdateOptions{})
		return err
	case Namespace:
		_, err := c.Clientset.CoreV1().Namespaces().Update(context.TODO(), resource.(*corev1.Namespace), metav1.UpdateOptions{})
		return err
	case Service:
		_, err := c.Clientset.CoreV1().Services(namespace).Update(context.TODO(), resource.(*corev1.Service), metav1.UpdateOptions{})
		return err
	case Middleware:
		_, err := c.TraefikClientset.TraefikV1alpha1().Middlewares(namespace).Update(context.TODO(), resource.(*traefikv1alpha1.Middleware), metav1.UpdateOptions{})
		return err
	case IngressRoute:
		_, err := c.TraefikClientset.TraefikV1alpha1().IngressRoutes(namespace).Update(context.TODO(), resource.(*traefikv1alpha1.IngressRoute), metav1.UpdateOptions{})
		return err
	case IngressRouteTCP:
		_, err := c.TraefikClientset.TraefikV1alpha1().IngressRouteTCPs(namespace).Update(context.TODO(), resource.(*traefikv1alpha1.IngressRouteTCP), metav1.UpdateOptions{})
		return err
	case IngressRouteUDP:
		_, err := c.TraefikClientset.TraefikV1alpha1().IngressRouteUDPs(namespace).Update(context.TODO(), resource.(*traefikv1alpha1.IngressRouteUDP), metav1.UpdateOptions{})
		return err
	case TraefikService:
		_, err := c.TraefikClientset.TraefikV1alpha1().TraefikServices(namespace).Update(context.TODO(), resource.(*traefikv1alpha1.TraefikService), metav1.UpdateOptions{})
		return err
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func (c Cluster) UpdateCustomResource(namespace string, resource map[string]interface{}) error {
	apiVersion, ok := resource["apiVersion"].(string)
	if !ok {
		return fmt.Errorf("apiVersion not found or not a string")
	}

	kind, ok := resource["kind"].(string)
	if !ok {
		return fmt.Errorf("kind not found or not a string")
	}

	// Parse group and version
	var group, version string
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 2 {
		group = parts[0]
		version = parts[1]
	} else {
		group = ""
		version = parts[0]
	}

	// Get API group resources and create a REST mapper
	groupResources, err := restmapper.GetAPIGroupResources(c.DiscoveryClientset)
	if err != nil {
		return fmt.Errorf("failed to get API group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	// Get the REST mapping
	gvk := schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind,
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("failed to get REST mapping: %v", err)
	}

	// Create unstructured object
	unstructuredObj := &unstructured.Unstructured{
		Object: resource,
	}

	// Use the resource from the mapping
	_, err = c.DynamicClientset.Resource(mapping.Resource).
		Namespace(namespace).
		Update(context.TODO(), unstructuredObj, metav1.UpdateOptions{})

	if err != nil {
		return fmt.Errorf("failed to update resource: %v", err)
	}

	return nil
}
