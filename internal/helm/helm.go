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
		fmt.Printf("Failed to initialize Helm Client: %v", err)
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
		log.Fatalf("Error adding or updating chart repo: %v", err)
	}

	// Define the chart to be installed
	chartSpec := helmclient.ChartSpec{
		ReleaseName:     c.ReleaseName,
		ChartName:       c.ChartName,
		Wait:            c.Wait,
		UpgradeCRDs:     true,
		CreateNamespace: true,
		ValuesYaml:      c.Values,
	}

	// Install the chart
	if _, err := h.InstallOrUpgradeChart(context.Background(), &chartSpec, nil); err != nil {
		log.Fatalf("Error installing chart: %v", err)
	}
}