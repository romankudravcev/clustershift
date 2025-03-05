package kube

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlserializer "k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/restmapper"
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
	case ServiceAccount:
		_, err := c.Clientset.CoreV1().ServiceAccounts(namespace).Create(context.TODO(), resource.(*corev1.ServiceAccount), metav1.CreateOptions{})
		return err
	case ClusterRole:
		_, err := c.Clientset.RbacV1().ClusterRoles().Create(context.TODO(), resource.(*rbacv1.ClusterRole), metav1.CreateOptions{})
		return err
	case ClusterRoleBind:
		_, err := c.Clientset.RbacV1().ClusterRoleBindings().Create(context.TODO(), resource.(*rbacv1.ClusterRoleBinding), metav1.CreateOptions{})
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

func (c Cluster) CreateConfigmap(name string, namespace string, data map[string]string) {
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

	err := c.CreateResource(ConfigMap, name, namespace, configMap)
	if err != nil {
		//TODO error handling
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

	// Create decoder
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(content), 4096)

	// Get REST mapper
	groupResources, err := restmapper.GetAPIGroupResources(c.DiscoveryClientset)
	if err != nil {
		return fmt.Errorf("failed to get API group resources: %v", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	// Process each document in the YAML
	for {
		var rawObj runtime.RawExtension
		if err := decoder.Decode(&rawObj); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error decoding YAML: %v", err)
		}

		// Decode YAML to unstructured
		obj := &unstructured.Unstructured{}
		yamlSerializer := yamlserializer.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
		_, gvk, err := yamlSerializer.Decode(rawObj.Raw, nil, obj)
		if err != nil {
			return fmt.Errorf("failed to decode manifest: %v", err)
		}

		// Find GVR
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("failed to get REST mapping for %s: %v", gvk.Kind, err)
		}

		// Get namespace
		namespace := obj.GetNamespace()
		if namespace == "" {
			namespace = "default"
		}

		// Prepare the dynamic resource interface
		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			dr = c.DynamicClientset.Resource(mapping.Resource).Namespace(namespace)
		} else {
			dr = c.DynamicClientset.Resource(mapping.Resource)
		}

		// Server side apply
		_, err = dr.Patch(context.Background(),
			obj.GetName(),
			types.ApplyPatchType,
			rawObj.Raw,
			metav1.PatchOptions{
				FieldManager: "clustershift",
			})

		if err != nil {
			return fmt.Errorf("failed to apply %s %s: %v", gvk.Kind, obj.GetName(), err)
		}

		//fmt.Printf("Successfully created %s/%s in namespace %s\n",
		//    gvk.Kind, obj.GetName(), namespace)
	}

	return nil
}

func (c Cluster) CreateCustomResource(namespace string, resource map[string]interface{}) error {
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
		Create(context.TODO(), unstructuredObj, metav1.CreateOptions{})

	if err != nil {
		return fmt.Errorf("failed to create resource: %v", err)
	}

	return nil
}
