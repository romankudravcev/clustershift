package constants

const (
	// Debug flag helm
	Debug = true

	// Conectivity probe constants
	ConnectivityProbeDeploymentURL  = "https://raw.githubusercontent.com/romankudravcev/kube-connectivity-probe/main/infra/Deployment.yml"
	ConnectivityProbeConfigmapURL   = "https://raw.githubusercontent.com/romankudravcev/kube-connectivity-probe/main/infra/Configmap.yml"
	ConnectivityProbeDeploymentName = "kube-connectivity-probe"
	ConnectivityProbeLabelSelector  = "app=" + ConnectivityProbeDeploymentName
	ConnectivityProbeConfigmapName  = "connectivity-config"
	ConnectivityProbeNamespace      = "connectivity-probe"
	ConnectivityProbePort           = 6443

	// Proxy constants
	HttpProxyURL       = "https://raw.githubusercontent.com/romankudravcev/reverse-proxy-http/main/Deployment.yml"
	HttpProxyPort      = 8743
	HttpProxyNamespace = "proxy"

	// Submariner constants
	SubmarinerRepoName          = "submariner-latest"
	SubmarinerRepoURL           = "https://submariner-io.github.io/submariner-charts/charts"
	SubmarinerBrokerChartName   = "submariner-latest/submariner-k8s-broker"
	SubmarinerOperatorChartName = "submariner-latest/submariner-operator"
	SubmarinerBrokerNamespace   = "submariner-k8s-broker"
	SubmarinerOperatorNamespace = "submariner-operator"
	SubmarinerBrokerClientToken = "submariner-k8s-broker-client-token"
	SubmarinerVersion           = "0.19"

	// CNPG constants
	CNPGOperatorURL   = "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.24/releases/cnpg-1.24.1.yaml"
	CNPGNamespace     = "cnpg-system"
	CNPGLabelSelector = "app.kubernetes.io/name=cloudnative-pg"
)
