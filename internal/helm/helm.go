package helm

import (
	"context"
	"fmt"
	"log"
	"os"

	helmclient "github.com/mittwald/go-helm-client"
	"helm.sh/helm/v3/pkg/repo"
)

func GetHelmClient(h HelmClientOptions) helmclient.Client {
	// Read kubeconfig file
	kubeConfig, err := os.ReadFile(h.kubeConfigPath)

	opt := &helmclient.KubeConfClientOptions{
		Options: &helmclient.Options{
			Namespace:        h.namespace,
			RepositoryCache:  "/tmp/.helmcache",
			RepositoryConfig: "/tmp/.helmrepo",
			Debug:            h.debug,
		},
		KubeContext: h.context,
		KubeConfig:  []byte(kubeConfig),
	}

	// Initialize Helm Client
	helmClient, err := helmclient.NewClientFromKubeConf(opt)

	if err != nil {
		fmt.Printf("Failed to initialize Helm Client: %v", err)
		return nil
	}
	return helmClient
}

func helmAddandInstallChart(h helmclient.Client, c ChartOptions) {
	chartRepo := repo.Entry{
		Name: c.repoName,
		URL:  c.repoURL,
	}

	// Add the chart repository
	if err := h.AddOrUpdateChartRepo(chartRepo); err != nil {
		log.Fatalf("Error adding or updating chart repo: %v", err)
	}

	// Define the chart to be installed
	chartSpec := helmclient.ChartSpec{
		ReleaseName:     c.releaseName,
		ChartName:       c.chartName,
		Wait:            c.wait,
		UpgradeCRDs:     true,
		CreateNamespace: true,
	}

	// Install the chart
	if _, err := h.InstallOrUpgradeChart(context.Background(), &chartSpec, nil); err != nil {
		log.Fatalf("Error installing chart: %v", err)
	}
}
