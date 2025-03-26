package redirect

import (
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"fmt"
	"strconv"

	v1 "k8s.io/api/core/v1"
)

func InitializeRequestForwarding(c kube.Clusters) {
	logger.Info("Deploy reverse proxy for request forwarding")

	// Get the Loadbalancer IP of the target cluster
	logger.Debug("Fetching loadbalancer IP")
	ip, err := getLoadbalancerIP(c.Target)
	exit.OnErrorWithMessage(err, "Failed to get loadbalancer ip")
	logger.Debug(fmt.Sprintf("Fetched loadbalancer IP: %s", ip))

	// Create HTTP proxy resources in the origin cluster
	logger.Debug("Deploying proxy")
	createHttpProxyDeployment(c.Origin, ip)
}

func EnableRequestForwarding(c kube.Clusters) {
	logger.Info("Enable request forwarding from origin")
	err := c.Origin.CreateResourcesFromURL(constants.HttpProxyIngressURL, "")
	exit.OnErrorWithMessage(err, "Failed to create resources from URL")
}

func getLoadbalancerIP(c kube.Cluster) (string, error) {
	services, err := c.FetchResources(kube.Service)
	if err != nil {
		return "", fmt.Errorf("fetching services for loadbalancer ip failed: %v", err)
	}

	serviceList, ok := services.(*v1.ServiceList)
	if !ok {
		return "", fmt.Errorf("failed to cast resources to *v1.ServiceList")
	}

	for _, service := range serviceList.Items {
		if service.Status.LoadBalancer.Ingress != nil && len(service.Status.LoadBalancer.Ingress) > 0 {
			ip := service.Status.LoadBalancer.Ingress[0].IP
			logger.Debug(fmt.Sprintf("Service: %s IP: %s", service.ObjectMeta.Name, ip))
			return ip, nil
		}
	}
	return "", fmt.Errorf("no LoadBalancer IP found")
}

func createHttpProxyDeployment(c kube.Cluster, lbIpTarget string) {
	// Create configmap
	data := map[string]string{
		"TARGET_URL": lbIpTarget,
		"PORT":       strconv.Itoa(constants.HttpProxyPort),
	}
	c.CreateConfigmap("http-proxy-config", constants.HttpProxyNamespace, data)

	// Create resources from yaml
	err := c.CreateResourcesFromURL(constants.HttpProxyDeploymentURL, "")
	exit.OnErrorWithMessage(err, "Failed to create resources from URL")
}
