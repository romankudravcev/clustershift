package redirect

import (
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"clustershift/internal/migration"
	"clustershift/internal/prompt"
	"clustershift/pkg/linkerd"
	"fmt"
	traefikv1dynamic "github.com/traefik/traefik/v3/pkg/config/dynamic"
	traefikv1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Redirect(c kube.Clusters, migrationResource migration.Resources, opts prompt.MigrationOptions) {
	//err := exportAllServices(c, migrationResource)
	//exit.OnErrorWithMessage(err, "Failed to export all services")
	err := updateIngressRoutes(c.Origin, migrationResource, opts)
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

		if migrationResource.GetNetworkingTool() == prompt.NetworkingToolSubmariner {
			err = createRemoteService(c.Origin, migrationResource, service)
			if err != nil {
				return fmt.Errorf("failed to create remote service: %v", err)
			}
		}
	}

	return nil
}

// updateIngressRoutes gets all IngressRoutes of origin and changes the service name to the exported service name
func updateIngressRoutes(c kube.Cluster, migrationResource migration.Resources, opts prompt.MigrationOptions) error {
	if migrationResource.GetNetworkingTool() == prompt.NetworkingToolLinkerd {
		namespaceObj, err := c.FetchResource(kube.Namespace, "traefik", "")
		if err != nil {
			return fmt.Errorf("fetching traefik namespace failed: %v", err)
		}
		namespace := namespaceObj.(*v1.Namespace)
		err = c.AddAnnotation(namespace, "linkerd.io/inject", "ingress")
		exit.OnErrorWithMessage(err, "Failed to add linkerd inject annotation to namespace")
		err = linkerd.RerollPodsInNamespace(c, namespace.Name)
		exit.OnErrorWithMessage(err, "Failed to reroll pods in namespace "+namespace.Name)
	}

	ingressRoutes, err := c.FetchResources(kube.IngressRoute)
	if err != nil {
		return fmt.Errorf("fetching ingress routes for update failed: %v", err)
	}

	ingressRouteList, ok := ingressRoutes.(*traefikv1.IngressRouteList)
	if !ok {
		return fmt.Errorf("failed to cast resources to *v1.IngressRouteList")
	}

	for _, ingressRoute := range ingressRouteList.Items {
		if ingressRoute.Name == "traefik-dashboard" {
			logger.Debug(fmt.Sprintf("Ignoring IngressRoute %s as it is the Traefik dashboard", ingressRoute.Name))
			continue
		}

		// replace the service name with the exported service name
		for i, route := range ingressRoute.Spec.Routes {
			for j, service := range route.Services {
				if migrationResource.GetNetworkingTool() == prompt.NetworkingToolSubmariner {
					// For Submariner, we need to use the remote service name
					remoteServiceName := service.Name + "-remote"
					ingressRoute.Spec.Routes[i].Services[j].Name = remoteServiceName
				} else if migrationResource.GetNetworkingTool() == prompt.NetworkingToolSkupper {
					remoteServiceName := service.Name + "-target"
					ingressRoute.Spec.Routes[i].Services[j].Name = remoteServiceName
				} else if migrationResource.GetNetworkingTool() == prompt.NetworkingToolLinkerd {
					remoteServiceName := service.Name + "-target"
					logger.Info(fmt.Sprintf("Updating service name in IngressRoute %s from %s to %s", ingressRoute.Name, service.Name, remoteServiceName))
					ingressRoute.Spec.Routes[i].Services[j].Name = remoteServiceName

					if opts.Rerouting == prompt.ReroutingLinkerd {
						reroutingMiddleware := &traefikv1.Middleware{
							ObjectMeta: metav1.ObjectMeta{
								Name:      remoteServiceName + "-rerouting-middleware",
								Namespace: ingressRoute.Namespace,
							},
							Spec: traefikv1.MiddlewareSpec{
								Headers: &traefikv1dynamic.Headers{
									CustomRequestHeaders: map[string]string{
										"l5d-dst-override": fmt.Sprintf("%s.%s.svc.cluster.local:80", remoteServiceName, ingressRoute.Namespace),
									},
								},
							},
						}

						err = c.CreateResource(kube.Middleware, reroutingMiddleware.Namespace, reroutingMiddleware)
						exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to create middleware %s", reroutingMiddleware.Name))

						var nativeLB bool
						nativeLB = true
						ingressRoute.Spec.Routes[i].Services[j].NativeLB = &nativeLB
						ingressRoute.Spec.Routes[i].Middlewares = append(ingressRoute.Spec.Routes[i].Middlewares, traefikv1.MiddlewareRef{
							Name: remoteServiceName + "-rerouting-middleware",
						})
					}
				} else {
					// For other networking tools, we can keep the original service name
					ingressRoute.Spec.Routes[i].Services[j].Name = fmt.Sprintf("target.%s.%s.svc.clusterset.local", service.Name, ingressRoute.Namespace)
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
			ExternalName: fmt.Sprintf("target.%s.%s.svc.clusterset.local", service.Name, service.Namespace),
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

	err := c.CreateResource(kube.Service, remoteService.Namespace, remoteService)
	if err != nil {
		return fmt.Errorf("failed to create remote service %s: %v", service.Name, err)
	}

	return nil
}
