package submariner

import (
	"clustershift/internal/cluster"
	"clustershift/internal/constants"
	"clustershift/internal/helm"
	"clustershift/internal/logger"
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
		logger.Debug(fmt.Sprintf("Error generating joing args: %v", err))
	}

	chartOptions := helm.ChartOptions{
		RepoName:    constants.SubmarinerRepoName,
		RepoURL:     constants.SubmarinerRepoURL,
		ReleaseName: constants.SubmarinerOperatorNamespace,
		ChartName:   constants.SubmarinerOperatorChartName,
		Values:      values,
		Wait:        true,
		Version:     constants.SubmarinerVersion,
	}

	helm.HelmAddandInstallChart(helmClient, chartOptions)
}
