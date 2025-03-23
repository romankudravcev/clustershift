package linkerd

import (
	"clustershift/internal/cert"
	"clustershift/internal/cluster"
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/helm"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"fmt"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"time"
)

func Install(c kube.Clusters) {
	logger.Debug("Create Linkerd certificates")

	certs, err := cert.GenerateLinkerdCerts(8760 * time.Hour) // 1 year validity for issuer
	if err != nil {
		panic(fmt.Sprintf("Failed to generate Linkerd certificates: %v", err))
	}

	installCluster(c.Origin, *certs)
	installCluster(c.Target, *certs)

	LinkCluster(c.Origin, c.Target, "origin")
	LinkCluster(c.Target, c.Origin, "target")
}

func installCluster(c kube.Cluster, certs cert.LinkerdCerts) {
	logger.Info("Installing Linkerd")

	c.CreateNewNamespace(constants.LinkerdNamespace)

	logger.Debug("Install linkerd-crds")
	deployEdgeChart(c.ClusterOptions, constants.LinkerdCrdsChartName, "linkerd-crds", "")

	logger.Debug("Install Linkerd control plane")
	valuesMap := map[string]interface{}{
		"identityTrustAnchorsPEM": string(certs.TrustAnchorsPEM),
		"identity": map[string]interface{}{
			"issuer": map[string]interface{}{
				"tls": map[string]interface{}{
					"crtPEM": string(certs.IssuerCertPEM),
					"keyPEM": string(certs.IssuerKeyPEM),
				},
			},
		},
	}

	// Convert the map to a YAML string
	controlPlaneValues, err := yaml.Marshal(valuesMap)
	if err != nil {
		panic(fmt.Sprintf("Failed to marshal YAML: %v", err))
	}
	deployEdgeChart(c.ClusterOptions, constants.LinkerdControlPlaneChartName, "linkerd-control-plane", string(controlPlaneValues))

	logger.Debug("Install linkerd-multicluster")
	deployMulticlusterChart(c.ClusterOptions, constants.LinkerdMultiClusterChartName, "linkerd-multicluster", "")
}

func linkClusterDep(fromCluster kube.Cluster, toCluster kube.Cluster, fromClusterName string) {

	serviceInterface, err := fromCluster.FetchResource(kube.Service, "linkerd-gateway", "linkerd-multicluster")
	if err != nil {
		exit.OnErrorWithMessage(err, "Error fetching linkerd-gateway service")
	}
	linkerdGateway := serviceInterface.(*v1.Service)

	gatewayIP := linkerdGateway.Status.LoadBalancer.Ingress[0].IP

	values := fmt.Sprintf(`
targetClusterName: %s
targetClusterDomain: cluster.local
gatewayAddress: %s:4143
gatewayIdentity: gateway.linkerd.cluster.local
probeSpec.path: /probe
probeSpec.port: 4191
probeSpec.period: 60s
`, fromClusterName, gatewayIP)

	deployMulticlusterChart(toCluster.ClusterOptions, constants.LinkerdMultiClusterLinkChartName, "charts", values)
}

func deployEdgeChart(clusterOpts *cluster.ClusterOptions, chartName, releaseName, values string) {
	helmOptions := helm.HelmClientOptions{
		KubeConfigPath: clusterOpts.KubeconfigPath,
		Context:        clusterOpts.Context,
		Namespace:      constants.LinkerdNamespace,
		Debug:          constants.Debug,
	}

	helmClient := helm.GetHelmClient(helmOptions)

	chartOptions := helm.ChartOptions{
		RepoName:    constants.LinkerdEdgeRepoName,
		RepoURL:     constants.LinkerdEdgeRepoURL,
		Namespace:   constants.LinkerdNamespace,
		ReleaseName: releaseName,
		ChartName:   chartName,
		Values:      values,
		Wait:        true,
	}

	helm.HelmAddandInstallChart(helmClient, chartOptions)
}

func deployMulticlusterChart(clusterOpts *cluster.ClusterOptions, chartName, releaseName, values string) {
	helmOptions := helm.HelmClientOptions{
		KubeConfigPath: clusterOpts.KubeconfigPath,
		Context:        clusterOpts.Context,
		Namespace:      constants.LinkerdMultiClusterNamespace,
		Debug:          constants.Debug,
	}

	helmClient := helm.GetHelmClient(helmOptions)

	chartOptions := helm.ChartOptions{
		RepoName:    constants.LinkerdRepoName,
		RepoURL:     constants.LinkerdRepoURL,
		Namespace:   constants.LinkerdMultiClusterNamespace,
		ReleaseName: releaseName,
		ChartName:   chartName,
		Values:      values,
		Wait:        true,
	}

	helm.HelmAddandInstallChart(helmClient, chartOptions)
}

func Uninstall(c kube.Cluster) {
	helmOptions := helm.HelmClientOptions{
		KubeConfigPath: c.ClusterOptions.KubeconfigPath,
		Context:        c.ClusterOptions.Context,
		Namespace:      constants.LinkerdNamespace,
		Debug:          constants.Debug,
	}

	helmClient := helm.GetHelmClient(helmOptions)

	releases, err := helmClient.ListDeployedReleases()
	if err != nil {
		logger.Error("Error listing deployed releases: %v", err)
		return
	}

	for _, release := range releases {
		logger.Info(fmt.Sprintf("Release: %s, Chart: %s, Namespace: %s",
			release.Name,
			release.Chart.Metadata.Name,
			release.Namespace))
	}

	logger.Warning("Error uninstalling charts", helmClient.UninstallReleaseByName("charts"))
	logger.Warning("Error uninstalling linkerd-multicluster", helmClient.UninstallReleaseByName("linkerd-multicluster"))
	logger.Warning("Error uninstalling linkerd-control-plane", helmClient.UninstallReleaseByName("linkerd-control-plane"))
	logger.Warning("Error uninstalling linkerd-crds", helmClient.UninstallReleaseByName("linkerd-crds"))
}
