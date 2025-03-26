package migration

import (
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/kubeconfig"
	"clustershift/internal/logger"
	"clustershift/internal/prompt"
	"clustershift/pkg/connectivity"
	cnpg "clustershift/pkg/database"
	"clustershift/pkg/linkerd"
	"clustershift/pkg/redirect"
	"clustershift/pkg/submariner"
	"time"
)

func Migrate(kubeconfigOrigin string, kubeconfigTarget string, opts prompt.MigrationOptions) {
	clusters := initClusters(kubeconfigOrigin, kubeconfigTarget)

	clusters.Origin.CreateNewNamespace("clustershift")
	clusters.Target.CreateNewNamespace("clustershift")

	connectivity.RunClusterConnectivityProbe(clusters)
	redirect.InitializeRequestForwarding(clusters)

	establishSecureConnection(opts, clusters)

	migrateConfigurationResources(clusters)
	migrateDatabases(clusters)
	migrateKubernetesResources(clusters)

	cnpg.DemoteOriginCluster(clusters.Origin)
	cnpg.DisableReplication(clusters.Target)
	redirect.EnableRequestForwarding(clusters)
}

func migrateDatabases(clusters kube.Clusters) {
	logger.Info("Migrate cnpg databases")
	cnpg.InstallOperator(clusters.Target)
	err := kube.WaitForPodsReady(clusters.Target, constants.CNPGLabelSelector, constants.CNPGNamespace, 90*time.Second)
	exit.OnErrorWithMessage(err, "Failed to wait for CNPG pods to be ready")

	//TODO this is submariner specific!
	cnpg.AddClustersetDNS(clusters.Origin)
	cnpg.ExportRWServices(clusters.Origin)
	cnpg.CreateReplicaClusters(clusters)
}

func establishSecureConnection(opts prompt.MigrationOptions, clusters kube.Clusters) {
	logger.Info("Establishing secure connection between clusters")
	if opts.NetworkingTool == prompt.NetworkingToolSubmariner {
		submariner.InstallSubmariner(clusters)
	} else {
		linkerd.Install(clusters)
	}
	logger.Info("Secure connection established")
}

func migrateKubernetesResources(clusters kube.Clusters) {
	logger.Info("Migrating resources")
	clusters.CreateResourceDiff(kube.Deployment)
	clusters.CreateResourceDiff(kube.Ingress)
	clusters.CreateResourceDiff(kube.Service)
	clusters.CreateResourceDiff(kube.IngressRoute)
	clusters.CreateResourceDiff(kube.IngressRouteTCP)
	clusters.CreateResourceDiff(kube.IngressRouteUDP)
	clusters.CreateResourceDiff(kube.Middleware)
	clusters.CreateResourceDiff(kube.TraefikService)
}

func migrateConfigurationResources(clusters kube.Clusters) {
	logger.Info("Migrating configuration resources")
	clusters.CreateResourceDiff(kube.Namespace)
	clusters.CreateResourceDiff(kube.ConfigMap)
	clusters.CreateResourceDiff(kube.Secret)
	clusters.CreateResourceDiff(kube.ServiceAccount)
	clusters.CreateResourceDiff(kube.ClusterRole)
	clusters.CreateResourceDiff(kube.ClusterRoleBind)
}

func initClusters(kubeconfigOrigin string, kubeconfigTarget string) kube.Clusters {
	logger.Info("Initializing kubernetes clients")

	// Copy the kubeconfig files to a temporary directory and modify them
	exit.OnErrorWithMessage(kubeconfig.ProcessKubeconfig(kubeconfigOrigin, "origin"), "Processing Kubeconfig failed")
	exit.OnErrorWithMessage(kubeconfig.ProcessKubeconfig(kubeconfigTarget, "target"), "Processing Kubeconfig failed")

	// Initialize the kubernetes clients
	clusters, err := kube.InitClients(constants.KubeconfigOriginTmp, constants.KubeconfigTargetTmp)
	exit.OnErrorWithMessage(err, "Failed to initialize kubernetes clients")
	return clusters
}
