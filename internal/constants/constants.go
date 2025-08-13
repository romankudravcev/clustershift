package constants

const (
	// Debug flag helm
	Debug = true

	// Kubeconfig temp paths
	KubeconfigOriginTmp = "tmp/origin_kubeconfig.yaml"
	KubeconfigTargetTmp = "tmp/target_kubeconfig.yaml"

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
	SubmarinerVersion           = "0.20.1"

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

	// Skupper constants
	SkupperSiteControllerURL = "https://raw.githubusercontent.com/skupperproject/skupper/refs/heads/1.8/cmd/site-controller/deploy-watch-all-ns.yaml"

	// CNPG constants
	CNPGNamespace     = "cnpg-system"
	CNPGLabelSelector = "app.kubernetes.io/name=cloudnative-pg"

	// MongoDB Community Operator constants
	MongoDBOperatorRepoName  = "mongodb"
	MongoDBOperatorRepoURL   = "https://mongodb.github.io/helm-charts"
	MongoDBOperatorChartName = "community-operator"
	MongoSyncerURL           = "https://raw.githubusercontent.com/romankudravcev/mongosyncer/refs/heads/main/k8s-job.yaml"
)
