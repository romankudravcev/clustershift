package submariner

import (
	"clustershift/internal/cli"
	"clustershift/internal/constants"
	"clustershift/internal/kube"
	"encoding/base64"

	v1 "k8s.io/api/core/v1"
)

func InstallSubmariner(c kube.Clusters, logger *cli.Logger) {
	log := logger.Log("Installing Submariner")
	defer log.Success("Submariner installed")

	// Gather necessary information
	cidrs := BuildCIDRs(c)

	l := log.Log("Labeling gateway nodes")
	// Label one master node in each cluster as a gateway node
	LabelGatewayNode(c.Origin)
	LabelGatewayNode(c.Target)
	l.Success("Labeled gateway nodes")

	// Deploy broker
	l = log.Log("Deploying broker")
	DeployBroker(*c.Origin.ClusterOptions)
	l.Success("Deployed broker")

	psk := GenerateRandomString(64)
	secretInterface, err := c.Origin.FetchResource(kube.Secret, constants.SubmarinerBrokerClientToken, constants.SubmarinerBrokerNamespace)
	if err != nil {
		panic(err)
	}
	secret := secretInterface.(*v1.Secret)
	token := DecodeBase64String(base64.StdEncoding.EncodeToString(secret.Data["token"]))
	ca := base64.StdEncoding.EncodeToString(secret.Data["ca.crt"])

	originJoinOptions := SubmarinerJoinOptions{
		Psk:         psk,
		BrokerURL:   cidrs.brokerURL,
		Token:       token,
		CA:          ca,
		ClusterId:   "origin",
		PodCIDR:     cidrs.podCIDROrigin,
		ServiceCIDR: cidrs.serviceCIDROrigin,
		GlobalCIDR:  "242.0.0.0/16",
	}

	targetJoinOptions := SubmarinerJoinOptions{
		Psk:         psk,
		BrokerURL:   cidrs.brokerURL,
		Token:       token,
		CA:          ca,
		ClusterId:   "target",
		PodCIDR:     cidrs.podCIDRTarget,
		ServiceCIDR: cidrs.serviceCIDRTarget,
		GlobalCIDR:  "242.1.0.0/16",
	}

	// Deploy operator
	l = log.Log("Joining origin cluster")
	JoinCluster(*c.Origin.ClusterOptions, originJoinOptions)
	l.Success("Joined origin cluster")
	l = log.Log("Joining target cluster")
	JoinCluster(*c.Target.ClusterOptions, targetJoinOptions)
	l.Success("Joined target cluster")
}

func LabelGatewayNode(c kube.Cluster) {
	node := c.FetchMasterNode()
	labels := map[string]string{
		"submariner.io/gateway": "true",
	}

	c.AddNodeLabels(&node.Items[0], labels)
}
