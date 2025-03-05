package migration

import (
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/kubeconfig"
	"clustershift/internal/logger"
	"clustershift/pkg/connectivity"
	cnpg "clustershift/pkg/database"
	"clustershift/pkg/redirect"
	"clustershift/pkg/submariner"
	"fmt"
	"time"
)

func Migrate(kubeconfigOrigin string, kubeconfigTarget string) {
	//TODO extract code
	logger.Info("Initializing kubernetes clients")
	// Copy the kubeconfig files to a temporary directory and modify them
	exit.OnErrorWithMessage(kubeconfig.ProcessKubeconfig(kubeconfigOrigin, "origin"), "Processing Kubeconfig failed")
	exit.OnErrorWithMessage(kubeconfig.ProcessKubeconfig(kubeconfigTarget, "target"), "Processing Kubeconfig failed")

	// get new kubeconfig paths
	kubeconfigOrigin = "tmp/origin_kubeconfig.yaml"
	kubeconfigTarget = "tmp/target_kubeconfig.yaml"

	clusters, err := kube.InitClients(kubeconfigOrigin, kubeconfigTarget)
	if err != nil {
		fmt.Println(err)
		return
	}
	logger.Info("Initialized kubernetes clients")

	// Create clustershift namespace
	clusters.Origin.CreateNewNamespace("clustershift")
	clusters.Target.CreateNewNamespace("clustershift")

	// Check connectivity between clusters
	logger.Info("Checking connectivity between clusters")
	connectivity.RunClusterConnectivityProbe(clusters)
	logger.Info("Connectivity check complete")

	// Deploy reverse proxy
	logger.Info("Deploy reverse proxy for request forwarding")
	redirect.InitializeRequestForwarding(clusters)
	logger.Info("Reverse proxy deployed")

	// Create secure connection between clusters via submariner
	submariner.InstallSubmariner(clusters)

	logger.Info("Migrating configuration resources")
	clusters.CreateResourceDiff(kube.Namespace)
	clusters.CreateResourceDiff(kube.ConfigMap)
	clusters.CreateResourceDiff(kube.Secret)
	clusters.CreateResourceDiff(kube.ServiceAccount)
	clusters.CreateResourceDiff(kube.ClusterRole)
	clusters.CreateResourceDiff(kube.ClusterRoleBind)
	logger.Info("Configuration resources migrated")

	logger.Info("Migrate cnpg databases")
	cnpg.InstallOperator(clusters.Target)
	kube.WaitForPodsReady(clusters.Target, constants.CNPGLabelSelector, constants.CNPGNamespace, 90*time.Second)
	cnpg.AddClustersetDNS(clusters.Origin)
	cnpg.ExportRWServices(clusters.Origin)
	cnpg.CreateReplicaClusters(clusters)

	logger.Info("cnpg databases migrated")

	logger.Info("Migrating resources")
	clusters.CreateResourceDiff(kube.Deployment)
	clusters.CreateResourceDiff(kube.Ingress)
	clusters.CreateResourceDiff(kube.Service)
	clusters.CreateResourceDiff(kube.IngressRoute)
	clusters.CreateResourceDiff(kube.IngressRouteTCP)
	clusters.CreateResourceDiff(kube.IngressRouteUDP)
	clusters.CreateResourceDiff(kube.Middleware)
	clusters.CreateResourceDiff(kube.TraefikService)
	logger.Info("Resources migrated")

	// Demote and promote databases
	cnpg.DemoteOriginCluster(clusters.Origin)
	cnpg.DisableReplication(clusters.Target)

	logger.Info("Enable request forwarding from origin")
	redirect.EnableRequestForwarding(clusters)
	logger.Info("Established request forwarding")

	logger.Info("Migration complete")
}
