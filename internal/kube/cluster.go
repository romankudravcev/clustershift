package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (c Cluster) FetchMasterNode() *v1.NodeList {
	nodes, err := c.Clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master",
	})
	if err != nil {
		panic(err.Error())
	}

	if len(nodes.Items) == 0 {
		//log.Fatal("No master node found")
	}

	return nodes
}

func (c Cluster) AddNodeLabels(node *v1.Node, labels map[string]string) {
	// Create a patch with the new labels
	patchLabels := map[string]interface{}{"metadata": map[string]interface{}{"labels": labels}}
	patchBytes, err := json.Marshal(patchLabels)
	if err != nil {
		panic(err.Error())
	}

	// Apply the patch
	_, err = c.Clientset.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		panic(err.Error())
	}
}

func (c Cluster) AddLabel(resourceType ResourceType, name, namespace string, labels map[string]string) error {
	// Create a patch with the new labels
	patchLabels := map[string]interface{}{"metadata": map[string]interface{}{"labels": labels}}
	patchBytes, err := json.Marshal(patchLabels)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %v", err)
	}

	// Apply the patch based on resource type
	switch resourceType {
	case Deployment:
		_, err = c.Clientset.AppsV1().Deployments(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case ConfigMap:
		_, err = c.Clientset.CoreV1().ConfigMaps(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case Ingress:
		_, err = c.Clientset.NetworkingV1().Ingresses(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case Secret:
		_, err = c.Clientset.CoreV1().Secrets(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case Namespace:
		_, err = c.Clientset.CoreV1().Namespaces().Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case Service:
		_, err = c.Clientset.CoreV1().Services(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case ServiceAccount:
		_, err = c.Clientset.CoreV1().ServiceAccounts(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case ClusterRole:
		_, err = c.Clientset.RbacV1().ClusterRoles().Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case ClusterRoleBind:
		_, err = c.Clientset.RbacV1().ClusterRoleBindings().Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case Middleware:
		_, err = c.TraefikClientset.TraefikV1alpha1().Middlewares(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case IngressRoute:
		_, err = c.TraefikClientset.TraefikV1alpha1().IngressRoutes(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case IngressRouteTCP:
		_, err = c.TraefikClientset.TraefikV1alpha1().IngressRouteTCPs(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case IngressRouteUDP:
		_, err = c.TraefikClientset.TraefikV1alpha1().IngressRouteUDPs(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case TraefikService:
		_, err = c.TraefikClientset.TraefikV1alpha1().TraefikServices(namespace).Patch(context.TODO(), name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	if err != nil {
		return fmt.Errorf("failed to patch resource: %v", err)
	}
	return nil
}

func (c Cluster) AddAnnotation(resource metav1.Object, annotationKey, annotationValue string) error {
	// Create a patch with the new annotation
	var patchBytes []byte
	var err error

	// Check if existing annotations are nil, if so, create a new map
	annotations := resource.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Add or update the annotation
	annotations[annotationKey] = annotationValue

	// Create a patch with the updated annotations
	patchAnnotations := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	}

	patchBytes, err = json.Marshal(patchAnnotations)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %v", err)
	}

	// Determine the type of resource and patch accordingly
	switch r := resource.(type) {
	case *v1.Node:
		_, err = c.Clientset.CoreV1().Nodes().Patch(context.TODO(), r.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case *appv1.Deployment:
		_, err = c.Clientset.AppsV1().Deployments(r.Namespace).Patch(context.TODO(), r.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case *v1.Pod:
		_, err = c.Clientset.CoreV1().Pods(r.Namespace).Patch(context.TODO(), r.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	case *v1.Service:
		_, err = c.Clientset.CoreV1().Services(r.Namespace).Patch(context.TODO(), r.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	default:
		return fmt.Errorf("unsupported resource type: %T", resource)
	}

	if err != nil {
		return fmt.Errorf("failed to patch resource: %v", err)
	}

	return nil
}
func (c Cluster) FetchServiceCIDRs() (string, error) {
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tst",
		},
		Spec: v1.ServiceSpec{
			ClusterIP: "1.1.1.1", // Use a specific IP
			Ports: []v1.ServicePort{
				{
					Port: 443,
				},
			},
		},
	}

	_, err := c.Clientset.CoreV1().Services("default").Create(context.TODO(), service, metav1.CreateOptions{})
	if err != nil {
		// Check if the error message contains the "valid IPs is" text
		if strings.Contains(err.Error(), "valid IPs is") {
			parts := strings.Split(err.Error(), "valid IPs is ")
			if len(parts) > 1 {
				return parts[1], nil
			}
		}
		return "", fmt.Errorf("error creating service: %v", err)
	}

	return "", nil // Return an empty string if no error occurs
}

func (c Cluster) FetchPodCIDRs() (string, error) {
	nodes, err := c.Clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master",
	})

	if err != nil {
		return "", fmt.Errorf("error retrieving nodes: %v", err)
	}

	for _, node := range nodes.Items {
		// Return the first podCIDR found on a master node
		if node.Spec.PodCIDR != "" {
			return node.Spec.PodCIDR, nil
		}
	}

	return "", fmt.Errorf("no podCIDR found for master nodes")
}

func (c Cluster) FetchKubernetesAPIEndpoint() (string, error) {
	// Retrieve the Endpoints object for "kubernetes" in the specified namespace
	endpoints, err := c.Clientset.CoreV1().Endpoints("default").Get(context.TODO(), "kubernetes", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error retrieving endpoints: %v", err)
	}

	// Check if subsets are present and extract the IP and port
	if len(endpoints.Subsets) > 0 && len(endpoints.Subsets[0].Addresses) > 0 {
		ip := endpoints.Subsets[0].Addresses[0].IP
		for _, port := range endpoints.Subsets[0].Ports {
			if port.Name == "https" {
				return fmt.Sprintf("%s:%d", ip, port.Port), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable IP and port found for the Kubernetes API endpoint")
}
