package migration

import (
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/kubeconfig"
	"clustershift/internal/logger"
	migration2 "clustershift/internal/migration"
	"clustershift/internal/prompt"
	"clustershift/pkg/connectivity"
	cnpg "clustershift/pkg/database"
	"clustershift/pkg/redirect"
	"time"
)

var clusters kube.Clusters
var resources migration2.Resources

func Migrate(kubeconfigOrigin string, kubeconfigTarget string, opts prompt.MigrationOptions) {
	prepareMigration(kubeconfigOrigin, kubeconfigTarget, opts)

	logger.Info("Establishing secure connection between clusters")
	resources.InstallNetworkingTool(clusters)

	migrateConfigurationResources()
	migrateDatabases(resources)
	migrateKubernetesResources()

	cnpg.DemoteOriginCluster(clusters.Origin)
	cnpg.DisableReplication(clusters.Target)
	redirect.EnableRequestForwarding(clusters)
}

func prepareMigration(kubeconfigOrigin string, kubeconfigTarget string, opts prompt.MigrationOptions) {
	initClusters(kubeconfigOrigin, kubeconfigTarget)

	var err error
	resources, err = migration2.GetMigrationResources(opts.NetworkingTool)
	exit.OnErrorWithMessage(err, "Unsupported networking tool")

	clusters.Origin.CreateNewNamespace("clustershift")
	clusters.Target.CreateNewNamespace("clustershift")

	connectivity.RunClusterConnectivityProbe(clusters)
	redirect.InitializeRequestForwarding(clusters)
}

func migrateDatabases(resources migration2.Resources) {
	logger.Info("Migrate cnpg databases")
	cnpg.InstallOperator(clusters.Target)
	err := kube.WaitForPodsReady(clusters.Target, constants.CNPGLabelSelector, constants.CNPGNamespace, 90*time.Second)
	exit.OnErrorWithMessage(err, "Failed to wait for CNPG pods to be ready")

	cnpg.AddClustersetDNS(clusters.Origin, resources)
	cnpg.ExportRWServices(clusters, clusters.Origin, resources)
	cnpg.CreateReplicaClusters(clusters)
}

func migrateKubernetesResources() {
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

func migrateConfigurationResources() {
	logger.Info("Migrating configuration resources")
	clusters.CreateResourceDiff(kube.Namespace)
	clusters.CreateResourceDiff(kube.ConfigMap)
	clusters.CreateResourceDiff(kube.Secret)
	clusters.CreateResourceDiff(kube.ServiceAccount)
	clusters.CreateResourceDiff(kube.ClusterRole)
	clusters.CreateResourceDiff(kube.ClusterRoleBind)
}

func initClusters(kubeconfigOrigin string, kubeconfigTarget string) {
	logger.Info("Initializing kubernetes clients")

	// Copy the kubeconfig files to a temporary directory and modify them
	exit.OnErrorWithMessage(kubeconfig.ProcessKubeconfig(kubeconfigOrigin, "origin"), "Processing Kubeconfig failed")
	exit.OnErrorWithMessage(kubeconfig.ProcessKubeconfig(kubeconfigTarget, "target"), "Processing Kubeconfig failed")

	// Initialize the kubernetes clients
	var err error
	clusters, err = kube.InitClients(constants.KubeconfigOriginTmp, constants.KubeconfigTargetTmp)
	exit.OnErrorWithMessage(err, "Failed to initialize kubernetes clients")
}
