package kube

import (
	traefikclientset "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type Cluster struct {
	Clientset        *kubernetes.Clientset
	TraefikClientset *traefikclientset.Clientset
	DynamicClientset dynamic.Interface
}

type Clusters struct {
	Origin Cluster
	Target Cluster
}
