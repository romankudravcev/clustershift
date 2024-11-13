package kube

import (
	"clustershift/internal/cluster"
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	traefikclientset "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned"
)

var clusters *Clusters

// InitClients initializes both Kubernetes clients from the given kubeconfig paths.
func InitClients(originKubeconfigPath, targetKubeconfigPath string) (Clusters, error) {
	originCluster, err := newCluster(originKubeconfigPath, "context-origin")
	if err != nil {
		return Clusters{}, fmt.Errorf("failed to initialize origin cluster: %w", err)
	}

	targetCluster, err := newCluster(targetKubeconfigPath, "context-target")
	if err != nil {
		return Clusters{}, fmt.Errorf("failed to initialize target cluster: %w", err)
	}

	clusters = &Clusters{
		Origin: *originCluster,
		Target: *targetCluster,
	}

	//fmt.Println("Successfully initialized Kubernetes clients")
	return *clusters, nil
}

func newCluster(kubeconfigPath string, context string) (*Cluster, error) {
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

	discoveryClientset, err := discovery.NewDiscoveryClientForConfig(config)

	clusterOptions := cluster.ClusterOptions{
		KubeconfigPath: kubeconfigPath,
		Context:        context,
	}

	return &Cluster{
		Clientset:          clientset,
		TraefikClientset:   traefikClientset,
		DynamicClientset:   dynamicClientset,
		DiscoveryClientset: discoveryClientset,
		ClusterOptions:     &clusterOptions,
	}, nil
}

// GetClusters returns the initialized origin and target Kubernetes clientset.
func GetClusters() *Clusters {
	return clusters
}
