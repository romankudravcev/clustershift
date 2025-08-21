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
	"clustershift/pkg/database/cnpg"
	mongooperator "clustershift/pkg/database/mongo/operator"
	mongostateful "clustershift/pkg/database/mongo/statefulset"
	"clustershift/pkg/database/postgres"
	"clustershift/pkg/linkerd"
	"clustershift/pkg/redirect"
	"clustershift/pkg/skupper"
	"fmt"
	v1 "k8s.io/api/core/v1"
)

var clusters kube.Clusters
var resources migration2.Resources

func Migrate(kubeconfigOrigin string, kubeconfigTarget string, opts prompt.MigrationOptions) {
	prepareMigration(kubeconfigOrigin, kubeconfigTarget, opts)

	logger.Info("Establishing secure connection between clusters")
	resources.InstallNetworkingTool(clusters)
	migrateConfigurationResources()

	if opts.Rerouting == prompt.ReroutingSkupper {
		handleSkupperRerouting()
	}

	if opts.Rerouting == prompt.ReroutingLinkerd {
		handleLinkerdRerouting()
	}

	migrateDatabases(resources, opts)
	migrateKubernetesResources()

	cnpg.DemoteOriginCluster(clusters.Origin)
	cnpg.DisableReplication(clusters.Target)
	redirect.EnableRequestForwarding(clusters, opts, resources)
}

func handleLinkerdRerouting() {
	namespaces, err := clusters.Target.FetchResources(kube.Namespace)

	exit.OnErrorWithMessage(err, "Failed to fetch namespaces from origin cluster")
	namespaceList, ok := namespaces.(*v1.NamespaceList)
	if !ok {
		exit.OnErrorWithMessage(fmt.Errorf("failed to convert to NamespaceList"), "Type assertion failed")
	}

	validNamespaces := filterValidNamespaces(namespaceList.Items)
	logger.Info(fmt.Sprintf("Number of valid namespaces found: %d", len(validNamespaces)))

	for _, namespace := range validNamespaces {
		if namespace.Name == "traefik" {
			err = clusters.Target.AddAnnotation(&namespace, "linkerd.io/inject", "ingress")
		} else {
			err = clusters.Target.AddAnnotation(&namespace, "linkerd.io/inject", "enabled")
		}
		exit.OnErrorWithMessage(err, "Failed to add linkerd inject annotation to namespace")
		err = linkerd.RerollPodsInNamespace(clusters.Target, namespace.Name)
		exit.OnErrorWithMessage(err, "Failed to reroll pods in namespace "+namespace.Name)
	}
}

func handleSkupperRerouting() {
	logger.Info("Entering Skupper rerouting section")

	namespaces, err := clusters.Origin.FetchResources(kube.Namespace)
	exit.OnErrorWithMessage(err, "Failed to fetch namespaces from origin cluster")
	namespaceList, ok := namespaces.(*v1.NamespaceList)
	if !ok {
		exit.OnErrorWithMessage(fmt.Errorf("failed to convert to NamespaceList"), "Type assertion failed")
	}

	// Filter namespaces to only include postgres, mongodb, and benchmark
	targetNamespaces := []string{"postgres", "mongodb", "benchmark"}
	validNamespaces := filterSpecificNamespaces(namespaceList.Items, targetNamespaces)
	logger.Info(fmt.Sprintf("Number of target namespaces found: %d", len(validNamespaces)))

	if len(validNamespaces) == 0 {
		logger.Info("No target namespaces (postgres, mongodb, benchmark) found in the cluster")
		return
	}

	for _, namespace := range validNamespaces {
		logger.Info("Creating Skupper site connection for namespace: " + namespace.Name)
		skupper.CreateSiteConnection(clusters, namespace.Name)
	}
	logger.Info("Finished processing all namespaces")
}

func prepareMigration(kubeconfigOrigin string, kubeconfigTarget string, opts prompt.MigrationOptions) {
	initClusters(kubeconfigOrigin, kubeconfigTarget)

	var err error
	resources, err = migration2.GetMigrationResources(opts.NetworkingTool)
	exit.OnErrorWithMessage(err, "Unsupported networking tool")
	clusters.Origin.CreateNewNamespace("clustershift")
	clusters.Target.CreateNewNamespace("clustershift")
	connectivity.RunClusterConnectivityProbe(clusters)
	if opts.Rerouting == prompt.ReroutingClustershift {
		redirect.InitializeRequestForwarding(clusters)
	}
}

func migrateDatabases(resources migration2.Resources, opts prompt.MigrationOptions) {
	cnpg.Migrate(clusters, resources, opts)
	mongostateful.Migrate(clusters, resources)
	mongooperator.Migrate(clusters, resources, opts)
	postgres.Migrate(clusters, resources)
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

func filterValidNamespaces(namespaces []v1.Namespace) []v1.Namespace {
	var validNamespaces []v1.Namespace

	// List of system namespaces to exclude
	excludeNamespaces := map[string]bool{
		"kube-system":             true,
		"kube-public":             true,
		"kube-node-lease":         true,
		"metallb-system":          true,
		"skupper-site-controller": true,
		"connectivity-probe":      true,
		"linkerd":                 true,
		"linkerd-multicluster":    true,
		"traefik":                 true,
	}

	for _, ns := range namespaces {
		// Skip terminating namespaces
		if ns.Status.Phase == v1.NamespaceTerminating {
			logger.Info("Skipping terminating namespace: " + ns.Name)
			continue
		}

		// Skip excluded system namespaces
		if excludeNamespaces[ns.Name] {
			logger.Info("Skipping system namespace: " + ns.Name)
			continue
		}

		logger.Info("Including namespace for Skupper: " + ns.Name)
		validNamespaces = append(validNamespaces, ns)
	}

	return validNamespaces
}

func filterSpecificNamespaces(namespaces []v1.Namespace, targetNamespaces []string) []v1.Namespace {
	var validNamespaces []v1.Namespace
	targetNamespaceMap := make(map[string]bool)

	// Create a map for quick lookup of target namespaces
	for _, ns := range targetNamespaces {
		targetNamespaceMap[ns] = true
	}

	for _, ns := range namespaces {
		// Skip terminating namespaces
		if ns.Status.Phase == v1.NamespaceTerminating {
			logger.Info("Skipping terminating namespace: " + ns.Name)
			continue
		}

		// Include only the namespaces that are in the targetNamespaces list
		if targetNamespaceMap[ns.Name] {
			logger.Info("Including namespace for Skupper: " + ns.Name)
			validNamespaces = append(validNamespaces, ns)
		}
	}

	return validNamespaces
}
