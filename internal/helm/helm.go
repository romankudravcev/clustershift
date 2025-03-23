package helm

import (
	"clustershift/internal/logger"
	"context"
	"fmt"
	"os"
	"time"

	helmclient "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/repo"
)

func GetHelmClient(h HelmClientOptions) helmclient.Client {
	// Read kubeconfig file
	kubeConfig, err := os.ReadFile(h.KubeConfigPath)

	opt := &helmclient.KubeConfClientOptions{
		Options: &helmclient.Options{
			Namespace:        h.Namespace,
			RepositoryCache:  "/tmp/.helmcache",
			RepositoryConfig: "/tmp/.helmrepo",
			Debug:            h.Debug,
		},
		KubeContext: h.Context,
		KubeConfig:  []byte(kubeConfig),
	}

	// Initialize Helm Client
	helmClient, err := helmclient.NewClientFromKubeConf(opt)

	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to initialize Helm Client: %v", err))
		return nil
	}
	return helmClient
}

func HelmAddandInstallChart(h helmclient.Client, c ChartOptions) {
	chartRepo := repo.Entry{
		Name: c.RepoName,
		URL:  c.RepoURL,
	}

	// Add the chart repository
	if err := h.AddOrUpdateChartRepo(chartRepo); err != nil {
		logger.Debug(fmt.Sprintf("Error adding or updating chart repo: %v", err))
	}

	// Define the chart to be installed
	chartSpec := helmclient.ChartSpec{
		ReleaseName:     c.ReleaseName,
		ChartName:       c.ChartName,
		Namespace:       c.Namespace,
		Wait:            c.Wait,
		UpgradeCRDs:     true,
		CreateNamespace: true,
		ValuesYaml:      c.Values,
		Version:         c.Version,
		Timeout:         120 * time.Second,
	}

	// Install the chart
	if _, err := h.InstallOrUpgradeChart(context.Background(), &chartSpec, nil); err != nil {
		logger.Debug(fmt.Sprintf("Error installing chart: %v", err))
	}
}
