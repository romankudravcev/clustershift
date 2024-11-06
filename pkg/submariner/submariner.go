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
	}

	helm.HelmAddandInstallChart(helmClient, chartOptions)
}

func JoinCluster(c cluster.ClusterOptions, args string) {
	helmOptions := helm.HelmClientOptions{
		KubeConfigPath: c.KubeconfigPath,
		Context:        c.Context,
		Namespace:      constants.SubmarinerOperatorNamespace,
		Debug:          constants.Debug,
	}

	helmClient := helm.GetHelmClient(helmOptions)
	//TODO - Generate values
	values := ""

	chartOptions := helm.ChartOptions{
		RepoName:    constants.SubmarinerRepoName,
		RepoURL:     constants.SubmarinerRepoURL,
		ReleaseName: constants.SubmarinerBrokerNamespace,
		ChartName:   constants.SubmarinerBrokerChartName,
		Values:      values,
		Wait:        true,
	}

	helm.HelmAddandInstallChart(helmClient, chartOptions)
}
