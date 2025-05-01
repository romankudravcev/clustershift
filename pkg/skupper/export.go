package skupper

import (
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	v1 "k8s.io/api/core/v1"
)

func ExportService(c kube.Cluster, namespace string, name string) {
	logger.Info("Export service")
	serviceInterface, err := c.FetchResource(kube.Service, name, namespace)
	exit.OnErrorWithMessage(err, "Could not fetch service")
	service := serviceInterface.(*v1.Service)
	exit.OnErrorWithMessage(c.AddAnnotation(service, "skupper.io/proxy", "tcp"), "Failed to annotate service")
	exit.OnErrorWithMessage(c.AddAnnotation(service, "skupper.io/address", name+c.Name), "Failed to annotate service")
}
