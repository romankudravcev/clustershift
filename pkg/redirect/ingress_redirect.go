package redirect

import (
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/migration"
	"fmt"
	taefikv1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Redirect(c kube.Clusters, migrationResource migration.Resources) {
	err := exportAllServices(c, migrationResource)
	exit.OnErrorWithMessage(err, "Failed to export all services")
	err = updateIngressRoutes(c.Origin, migrationResource)
	exit.OnErrorWithMessage(err, "Failed to update ingress routes")
}

// exportAllServices gets all services of target cluster and exports them
func exportAllServices(c kube.Clusters, migrationResource migration.Resources) error {
	services, err := c.Target.FetchResources(kube.Service)
	if err != nil {
		return fmt.Errorf("fetching services for export failed: %v", err)
	}

	serviceList, ok := services.(*v1.ServiceList)
	if !ok {
		return fmt.Errorf("failed to cast resources to *v1.ServiceList")
	}

	for _, service := range serviceList.Items {
		migrationResource.ExportService(c.Target, service.Namespace, service.Name)

		if migrationResource.GetNetworkingTool() == "submariner" {
			err = createRemoteService(c.Origin, migrationResource, service)
			if err != nil {
				return fmt.Errorf("failed to create remote service: %v", err)
			}
		}
	}

	return nil
}

// updateIngressRoutes gets all IngressRoutes of origin and changes the service name to the exported service name
func updateIngressRoutes(c kube.Cluster, migrationResource migration.Resources) error {
	ingressRoutes, err := c.FetchResources(kube.IngressRoute)
	if err != nil {
		return fmt.Errorf("fetching ingress routes for update failed: %v", err)
	}

	ingressRouteList, ok := ingressRoutes.(*taefikv1.IngressRouteList)
	if !ok {
		return fmt.Errorf("failed to cast resources to *v1.IngressRouteList")
	}

	for _, ingressRoute := range ingressRouteList.Items {
		// replace the service name with the exported service name
		for i, route := range ingressRoute.Spec.Routes {
			for j, service := range route.Services {
				if migrationResource.GetNetworkingTool() == "submariner" {
					// For Submariner, we need to use the remote service name
					remoteServiceName := service.Name + "-remote"
					ingressRoute.Spec.Routes[i].Services[j].Name = remoteServiceName
				} else {
					// For other networking tools, we can keep the original service name
					ingressRoute.Spec.Routes[i].Services[j].Name = migrationResource.GetDNSName(service.Name, ingressRoute.Namespace)
				}
			}
		}
		// Update the ingress route with the modified service names
		err = c.UpdateResource(kube.IngressRoute, ingressRoute.Name, ingressRoute.Namespace, &ingressRoute)
		if err != nil {
			return fmt.Errorf("failed to update ingress route %s: %v", ingressRoute.Name, err)
		}
	}

	return nil
}

func createRemoteService(c kube.Cluster, migrationResource migration.Resources, service v1.Service) error {
	// Create a new service in the origin cluster that points to the target cluster's service
	remoteService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      service.Name + "-remote",
			Namespace: service.Namespace,
		},
		Spec: v1.ServiceSpec{
			Type:         v1.ServiceTypeExternalName,
			ExternalName: migrationResource.GetDNSName(service.Name, service.Namespace),
			Ports: []v1.ServicePort{
				{
					Name:     "http",
					Port:     80,
					Protocol: v1.ProtocolTCP,
				},
			},
			Selector: map[string]string{"app": service.Name},
		},
	}

	err := c.CreateResource(kube.Service, remoteService.Name, remoteService.Namespace, remoteService)
	if err != nil {
		return fmt.Errorf("failed to create remote service %s: %v", service.Name, err)
	}

	return nil
}
