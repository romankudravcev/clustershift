package kube

import (
	"fmt"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	traefikclientset "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned"
)

var clusters *Clusters

// InitClients initializes both Kubernetes clients from the given kubeconfig paths.
func InitClients(originKubeconfigPath, targetKubeconfigPath string) (Clusters, error) {
	originCluster, err := newCluster(originKubeconfigPath)
	if err != nil {
		return Clusters{}, fmt.Errorf("failed to initialize origin cluster: %w", err)
	}

	targetCluster, err := newCluster(targetKubeconfigPath)
	if err != nil {
		return Clusters{}, fmt.Errorf("failed to initialize target cluster: %w", err)
	}

	clusters = &Clusters{
		Origin: *originCluster,
		Target: *targetCluster,
	}

	fmt.Println("Successfully initialized Kubernetes clients")
	return *clusters, nil
}

func newCluster(kubeconfigPath string) (*Cluster, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize clientset: %w", err)
	}

	traefikClientset, err := traefikclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize traefik clientset: %w", err)
	}

	dynamicClientset, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize dynamic clientset: %w", err)
	}

	return &Cluster{
		Clientset:        clientset,
		TraefikClientset: traefikClientset,
		DynamicClientset: dynamicClientset,
	}, nil
}

// GetClusters returns the initialized origin and target Kubernetes clientset.
func GetClusters() *Clusters {
	return clusters
}
