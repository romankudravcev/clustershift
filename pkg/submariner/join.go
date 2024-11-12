package submariner

import (
	"clustershift/internal/cluster"
	"clustershift/internal/constants"
	"clustershift/internal/helm"
	"fmt"
)

func JoinCluster(c cluster.ClusterOptions, s SubmarinerJoinOptions) {
	helmOptions := helm.HelmClientOptions{
		KubeConfigPath: c.KubeconfigPath,
		Context:        c.Context,
		Namespace:      constants.SubmarinerOperatorNamespace,
		Debug:          constants.Debug,
	}

	helmClient := helm.GetHelmClient(helmOptions)
	values, err := GenerateJoinArgs(s)
	if err != nil {
		fmt.Printf("Error generating joing args: %v", err)
	}

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
