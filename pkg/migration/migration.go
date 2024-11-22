package migration

import (
	"clustershift/internal/cli"
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/kubeconfig"
	"clustershift/pkg/connectivity"
	cnpg "clustershift/pkg/database"
	"clustershift/pkg/redirect"
	"clustershift/pkg/submariner"
	"fmt"
	"time"
)

func Migrate(kubeconfigOrigin string, kubeconfigTarget string) {
	logger := cli.NewLogger("Migrating cluster", nil)
	cli.SetOGLogger(logger)
	//TODO extract code
	l := logger.Log("Initializing kubernetes clients")
	// Copy the kubeconfig files to a temporary directory and modify them
	exit.OnErrorWithMessage(l.Fail("Processing Kubeconfig failed", kubeconfig.ProcessKubeconfig(kubeconfigOrigin, "origin")))
	exit.OnErrorWithMessage(l.Fail("Processing Kubeconfig failed", kubeconfig.ProcessKubeconfig(kubeconfigTarget, "target")))

	// get new kubeconfig paths
	kubeconfigOrigin = "tmp/origin_kubeconfig.yaml"
	kubeconfigTarget = "tmp/target_kubeconfig.yaml"

	clusters, err := kube.InitClients(kubeconfigOrigin, kubeconfigTarget)
	if err != nil {
		fmt.Println(err)
		return
	}
	l.Success("Initialized kubernetes clients")
	// Check connectivity between clusters
	l = logger.Log("Checking connectivity between clusters")
	connectivity.RunClusterConnectivityProbe(clusters, l)
	l.Success("Connectivity check complete")

	// Create secure connection between clusters via submariner
	submariner.InstallSubmariner(clusters, logger)

	l = logger.Log("Migrating configuration resources")
	clusters.CreateResourceDiff(kube.Namespace)
	clusters.CreateResourceDiff(kube.ConfigMap)
	clusters.CreateResourceDiff(kube.Secret)
	l.Success("Configuration resources migrated")

	l = logger.Log("Migrate cnpg databases")
	cnpg.InstallOperator(clusters.Target, l)
	kube.WaitForPodsReady(clusters.Target, constants.CNPGLabelSelector, constants.CNPGNamespace, 90*time.Second)
	cnpg.CreateReplicaClusters(clusters, l)
	cnpg.AddClustersetDNS(clusters.Origin, l)
	cnpg.ExportRWServices(clusters.Origin, l)
	l.Success("cnpg databases migrated")

	l = logger.Log("Migrating resources")
	clusters.CreateResourceDiff(kube.Deployment)
	clusters.CreateResourceDiff(kube.Ingress)
	clusters.CreateResourceDiff(kube.Service)
	clusters.CreateResourceDiff(kube.IngressRoute)
	clusters.CreateResourceDiff(kube.IngressRouteTCP)
	clusters.CreateResourceDiff(kube.IngressRouteUDP)
	clusters.CreateResourceDiff(kube.Middleware)
	clusters.CreateResourceDiff(kube.TraefikService)
	l.Success("Resources migrated")

	l = logger.Log("Redirect request from origin")
	redirect.InitializeRequestForwarding(clusters, l)
	l.Success("Established request forwarding")

	logger.Success("Migration complete")
}
