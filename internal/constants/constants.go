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
	HttpProxyDeploymentURL = "https://raw.githubusercontent.com/romankudravcev/reverse-proxy-http/main/Deployment.yml"
	HttpProxyIngressURL    = "https://raw.githubusercontent.com/romankudravcev/reverse-proxy-http/main/Ingress.yml"
	HttpProxyPort          = 8734
	HttpProxyNamespace     = "clustershift"

	// Submariner constants
	SubmarinerRepoName          = "submariner-latest"
	SubmarinerRepoURL           = "https://submariner-io.github.io/submariner-charts/charts"
	SubmarinerBrokerChartName   = "submariner-latest/submariner-k8s-broker"
	SubmarinerOperatorChartName = "submariner-latest/submariner-operator"
	SubmarinerBrokerNamespace   = "submariner-k8s-broker"
	SubmarinerOperatorNamespace = "submariner-operator"
	SubmarinerBrokerClientToken = "submariner-k8s-broker-client-token"
	SubmarinerVersion           = "0.19"

	// Linkerd constants
	LinkerdEdgeRepoName              = "linkerd-edge"
	LinkerdRepoName                  = "linkerd"
	LinkerdEdgeRepoURL               = "https://helm.linkerd.io/edge"
	LinkerdRepoURL                   = "https://helm.linkerd.io/stable"
	LinkerdCrdsChartName             = "linkerd-edge/linkerd-crds"
	LinkerdControlPlaneChartName     = "linkerd-edge/linkerd-control-plane"
	LinkerdMultiClusterChartName     = "linkerd/linkerd-multicluster"
	LinkerdMultiClusterLinkChartName = "linkerd/charts"
	LinkerdNamespace                 = "linkerd"
	LinkerdMultiClusterNamespace     = "linkerd-multicluster"

	// CNPG constants
	CNPGOperatorURL   = "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.24/releases/cnpg-1.24.1.yaml"
	CNPGNamespace     = "cnpg-system"
	CNPGLabelSelector = "app.kubernetes.io/name=cloudnative-pg"
)
