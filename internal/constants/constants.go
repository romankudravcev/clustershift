package constants

const (
	// Conectivity probe constants
	ConnectivityProbeDeploymentURL  = "https://raw.githubusercontent.com/romankudravcev/kube-connectivity-probe/main/infra/Deployment.yml"
	ConnectivityProbeConfigmapURL   = "https://raw.githubusercontent.com/romankudravcev/kube-connectivity-probe/main/infra/Configmap.yml"
	ConnectivityProbeDeploymentName = "kube-connectivity-probe"
	ConnectivityProbeConfigmapName  = "connectivity-config"
	ConnectivityProbeNamespace      = "connectivity-probe"
	ConnectivityProbePort           = 6443

	// Submariner constants
	SubmarinerRepoName          = "submariner-latest"
	SubmarinerRepoURL           = "https://submariner-io.github.io/submariner-charts/charts"
	SubmarinerBrokerChartName   = "submariner-latest/submariner-k8s-broker"
	SubmarinerOperatorChartName = "submariner-latest/submariner-operator"
	SubmarinerBrokerNamespace   = "submariner-k8s-broker"
	SubmarinerOperatorNamespace = "submariner-operator"
	SubmarinerBrokerClientToken = "submariner-k8s-broker-client-token"
	// Debug flag helm
	Debug = true
)
