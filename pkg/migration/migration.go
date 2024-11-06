package migration

import (
	"clustershift/internal/kube"
	"clustershift/internal/kubeconfig"
	"clustershift/pkg/submariner"
	"fmt"
)

func Migrate(kubeconfigOrigin string, kubeconfigTarget string) {
	// Copy the kubeconfig files to a temporary directory and modify them
	kubeconfig.ProcessKubeconfig(kubeconfigOrigin, "origin")
	kubeconfig.ProcessKubeconfig(kubeconfigTarget, "target")

	// get new kubeconfig paths
	kubeconfigOrigin = "tmp/origin_kubeconfig.yaml"
	kubeconfigTarget = "tmp/target_kubeconfig.yaml"

	clusters, err := kube.InitClients(kubeconfigOrigin, kubeconfigTarget)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Create secure connection between clusters via submariner
	submariner.InstallSubmariner(clusters)

	clusters.CreateResourceDiff(kube.Namespace)
	clusters.CreateResourceDiff(kube.ConfigMap)
	clusters.CreateResourceDiff(kube.Secret)
	clusters.CreateResourceDiff(kube.Deployment)
	clusters.CreateResourceDiff(kube.Ingress)
	clusters.CreateResourceDiff(kube.Service)
	clusters.CreateResourceDiff(kube.IngressRoute)
	clusters.CreateResourceDiff(kube.IngressRouteTCP)
	clusters.CreateResourceDiff(kube.IngressRouteUDP)
	clusters.CreateResourceDiff(kube.Middleware)
	clusters.CreateResourceDiff(kube.TraefikService)

	fmt.Println("Successfully migrated resources")
}
