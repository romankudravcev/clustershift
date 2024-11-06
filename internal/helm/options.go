package helm

type HelmClientOptions struct {
	kubeConfigPath string
	context        string
	namespace      string
	debug          bool
}

type ChartOptions struct {
	repoName    string
	repoURL     string
	releaseName string
	chartName   string
	wait        bool
}
