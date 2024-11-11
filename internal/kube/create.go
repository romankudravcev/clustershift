package kube

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
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

func (c Cluster) CreateNewNamespace(name string) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err := c.CreateResource(Namespace, "", "", namespace)
	if err != nil && err.Error() == "namespaces \""+name+"\" already exists" {
		// Log that namespace already exists
		//fmt.Printf("Namespace %s already exists\n", name)
	}
}

func (c Cluster) CreateResourcesFromURL(url string) error {
	// Register Traefik schemes
	_ = traefikv1alpha1.AddToScheme(scheme.Scheme)

	// Fetch YAML content
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch YAML from URL: %v", err)
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	// Create decoder and serializer
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(content), 4096)
	serializer := jsonserializer.NewSerializerWithOptions(
		jsonserializer.DefaultMetaFactory,
		scheme.Scheme,
		scheme.Scheme,
		jsonserializer.SerializerOptions{},
	)

	// Process each document in the YAML
	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("error decoding YAML: %v", err)
		}

		// Decode the raw object to get its type information
		obj, gvk, err := serializer.Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return fmt.Errorf("error decoding object: %v", err)
		}

		// Get namespace from the object
		metaObj, ok := obj.(metav1.Object)
		if !ok {
			return fmt.Errorf("object does not implement metav1.Object")
		}
		namespace := metaObj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		// Map Kubernetes kinds to your ResourceType
		var resourceType ResourceType
		var typedObj interface{}

		switch gvk.Kind {
		case "Deployment":
			deployment, ok := obj.(*appsv1.Deployment)
			if !ok {
				return fmt.Errorf("failed to convert object to Deployment")
			}
			resourceType = Deployment
			typedObj = deployment

		case "ConfigMap":
			configMap, ok := obj.(*corev1.ConfigMap)
			if !ok {
				return fmt.Errorf("failed to convert object to ConfigMap")
			}
			resourceType = ConfigMap
			typedObj = configMap

		case "Ingress":
			ingress, ok := obj.(*networkingv1.Ingress)
			if !ok {
				return fmt.Errorf("failed to convert object to Ingress")
			}
			resourceType = Ingress
			typedObj = ingress

		case "Secret":
			secret, ok := obj.(*corev1.Secret)
			if !ok {
				return fmt.Errorf("failed to convert object to Secret")
			}
			resourceType = Secret
			typedObj = secret

		case "Namespace":
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				return fmt.Errorf("failed to convert object to Namespace")
			}
			resourceType = Namespace
			typedObj = ns

		case "Service":
			service, ok := obj.(*corev1.Service)
			if !ok {
				return fmt.Errorf("failed to convert object to Service")
			}
			resourceType = Service
			typedObj = service

		case "Middleware":
			middleware, ok := obj.(*traefikv1alpha1.Middleware)
			if !ok {
				return fmt.Errorf("failed to convert object to Middleware")
			}
			resourceType = Middleware
			typedObj = middleware

		case "IngressRoute":
			ingressRoute, ok := obj.(*traefikv1alpha1.IngressRoute)
			if !ok {
				return fmt.Errorf("failed to convert object to IngressRoute")
			}
			resourceType = IngressRoute
			typedObj = ingressRoute

		case "IngressRouteTCP":
			ingressRouteTCP, ok := obj.(*traefikv1alpha1.IngressRouteTCP)
			if !ok {
				return fmt.Errorf("failed to convert object to IngressRouteTCP")
			}
			resourceType = IngressRouteTCP
			typedObj = ingressRouteTCP

		case "IngressRouteUDP":
			ingressRouteUDP, ok := obj.(*traefikv1alpha1.IngressRouteUDP)
			if !ok {
				return fmt.Errorf("failed to convert object to IngressRouteUDP")
			}
			resourceType = IngressRouteUDP
			typedObj = ingressRouteUDP

		case "TraefikService":
			traefikService, ok := obj.(*traefikv1alpha1.TraefikService)
			if !ok {
				return fmt.Errorf("failed to convert object to TraefikService")
			}
			resourceType = TraefikService
			typedObj = traefikService

		default:
			return fmt.Errorf("unsupported resource kind: %s", gvk.Kind)
		}

		// Create the resource
		err = c.CreateResource(resourceType, "", namespace, typedObj)
		if err != nil {
			return fmt.Errorf("failed to create %s: %v", gvk.Kind, err)
		}

		//fmt.Printf("Successfully created %s/%s in namespace %s\n",
		//	gvk.Kind, metaObj.GetName(), namespace)
	}

	return nil
}
