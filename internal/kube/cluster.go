package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
