package helm

type HelmClientOptions struct {
	KubeConfigPath string
	Context        string
	Namespace      string
	Debug          bool
}

type ChartOptions struct {
	RepoName    string
	RepoURL     string
	ReleaseName string
	ChartName   string
	Values      string
	Wait        bool
}
