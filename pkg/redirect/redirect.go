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
	// Get the Loadbalancer IP of the target cluster
	logger.Info("Fetching loadbalancer IP")
	ip, err := getLoadbalancerIP(c.Target)
	exit.OnErrorWithMessage(err, "Failed to get loadbalancer ip")
	logger.Info(fmt.Sprintf("Fetched loadbalancer IP: %s", ip))

	// Create HTTP proxy resources in the origin cluster
	logger.Info("Deploying proxy")
	createHttpProxyDeployment(c.Origin, ip)
	logger.Info("Deployed proxy")
}

func EnableRequestForwarding(c kube.Clusters) {
	logger.Info("Enable forwarding")
	c.Origin.CreateResourcesFromURL(constants.HttpProxyIngressURL)
	logger.Info("Forwarding enabled")
}

func getLoadbalancerIP(c kube.Cluster) (string, error) {
	services, err := c.FetchResources(kube.Service)
	if err != nil {
		return "", fmt.Errorf("Fetching services for loadbalancer ip failed: %v", err)
	}

	serviceList, ok := services.(*v1.ServiceList)
	if !ok {
		return "", fmt.Errorf("Failed to cast resources to *v1.ServiceList")
	}

	for _, service := range serviceList.Items {
		if service.Status.LoadBalancer.Ingress != nil && len(service.Status.LoadBalancer.Ingress) > 0 {
			ip := service.Status.LoadBalancer.Ingress[0].IP
			logger.Debug(fmt.Sprintf("Service: %s IP: %s", service.ObjectMeta.Name, ip))
			return ip, nil
		}
	}
	return "", fmt.Errorf("No LoadBalancer IP found")
}

func createHttpProxyDeployment(c kube.Cluster, lbIpTarget string) {
	// Create configmap
	data := map[string]string{
		"TARGET_URL": lbIpTarget,
		"PORT":       strconv.Itoa(constants.HttpProxyPort),
	}
	c.CreateConfigmap("http-proxy-config", constants.HttpProxyNamespace, data)

	// Create resources from yaml
	c.CreateResourcesFromURL(constants.HttpProxyDeploymentURL)
}

/*
func DeleteIngressRouteExceptProxy(c kube.Cluster) {
	// Fetch all ingress routes
	resources, err := c.FetchResources(kube.IngressRoute)
	if err != nil {
		//TODO error handling
	}

	ingressRoutes, ok := resources.(*traefikv1alpha1.IngressRouteList)
	if !ok {
		//TODO error handling
	}

	for _, ingressRoute := range ingressRoutes.Items {
		name := ingressRoute.Name
		namespace := ingressRoute.Namespace

		if namespace == "proxy" {
			continue
		}

		c.DeleteResource(kube.IngressRoute, name, namespace)
	}
}
*/
