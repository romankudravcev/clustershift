package submariner

import (
	"clustershift/internal/constants"
	"clustershift/internal/kube"
	"encoding/base64"

	v1 "k8s.io/api/core/v1"
)

func InstallSubmariner(c kube.Clusters) {
	// Gather necessary information
	cidrs := BuildCIDRs(c)

	// Label one master node in each cluster as a gateway node
	LabelGatewayNode(c.Origin)
	LabelGatewayNode(c.Target)

	// Deploy broker
	DeployBroker(*c.Origin.ClusterOptions)

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
	JoinCluster(*c.Origin.ClusterOptions, originJoinOptions)
	JoinCluster(*c.Target.ClusterOptions, targetJoinOptions)
}

func LabelGatewayNode(c kube.Cluster) {
	node := c.FetchMasterNode()
	labels := map[string]string{
		"submariner.io/gateway": "true",
	}

	c.AddNodeLabels(&node.Items[0], labels)
}
