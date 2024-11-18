package submariner

import (
	"clustershift/internal/cluster"
	"clustershift/internal/constants"
	"clustershift/internal/helm"
)

func DeployBroker(c cluster.ClusterOptions) {
	helmOptions := helm.HelmClientOptions{
		KubeConfigPath: c.KubeconfigPath,
		Context:        c.Context,
		Namespace:      constants.SubmarinerBrokerNamespace,
		Debug:          constants.Debug,
	}

	helmClient := helm.GetHelmClient(helmOptions)

	chartOptions := helm.ChartOptions{
		RepoName:    constants.SubmarinerRepoName,
		RepoURL:     constants.SubmarinerRepoURL,
		ReleaseName: constants.SubmarinerBrokerNamespace,
		ChartName:   constants.SubmarinerBrokerChartName,
		Wait:        true,
		Version:     constants.SubmarinerVersion,
	}

	helm.HelmAddandInstallChart(helmClient, chartOptions)
}
