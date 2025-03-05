package submariner

import (
	"clustershift/internal/constants"
	"clustershift/internal/decoder"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"encoding/base64"
	"fmt"

	v1 "k8s.io/api/core/v1"
)

func InstallSubmariner(c kube.Clusters) {
	logger.Info("Installing Submariner")
	defer logger.Info("Submariner installed")

	// Gather necessary information
	cidrs := BuildCIDRs(c)

	logger.Info("Labeling gateway nodes")
	// Label one master node in each cluster as a gateway node
	LabelGatewayNode(c.Origin)
	LabelGatewayNode(c.Target)
	logger.Info("Labeled gateway nodes")

	// Deploy broker
	logger.Info("Deploying broker")
	DeployBroker(*c.Origin.ClusterOptions)
	logger.Info("Deployed broker")

	psk := GenerateRandomString(64)
	secretInterface, err := c.Origin.FetchResource(kube.Secret, constants.SubmarinerBrokerClientToken, constants.SubmarinerBrokerNamespace)
	if err != nil {
		logger.Debug(fmt.Sprintf("Error fetching secret: %v", err))
		panic(err)
	}
	secret := secretInterface.(*v1.Secret)
	token := decoder.DecodeBase64String(base64.StdEncoding.EncodeToString(secret.Data["token"]))
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
	logger.Info("Joining origin cluster")
	JoinCluster(*c.Origin.ClusterOptions, originJoinOptions)
	logger.Info("Joined origin cluster")
	logger.Info("Joining target cluster")
	JoinCluster(*c.Target.ClusterOptions, targetJoinOptions)
	logger.Info("Joined target cluster")
}

func LabelGatewayNode(c kube.Cluster) {
	node := c.FetchMasterNode()
	labels := map[string]string{
		"submariner.io/gateway": "true",
	}

	c.AddNodeLabels(&node.Items[0], labels)
}
