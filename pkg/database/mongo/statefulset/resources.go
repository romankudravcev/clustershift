package statefulset

import (
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/migration"
	"clustershift/internal/mongo"
	"clustershift/internal/prompt"
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"strings"
)

const (
	mongoPort  = "27017"
	mongoImage = "mongo"
)

// scanExistingDatabases finds all MongoDB StatefulSets in the cluster
func scanExistingDatabases(c kube.Cluster) []appsv1.StatefulSet {
	statefulSets, err := c.Clientset.AppsV1().StatefulSets("").List(context.TODO(), metav1.ListOptions{})
	exit.OnErrorWithMessage(err, "Failed to list statefulsets")

	var matches []appsv1.StatefulSet
	for _, sts := range statefulSets.Items {
		if len(sts.OwnerReferences) > 0 && sts.OwnerReferences[0].Kind == "MongoDBCommunity" {
			continue
		}
		for _, container := range sts.Spec.Template.Spec.Containers {
			if strings.Contains(container.Image, mongoImage) {
				matches = append(matches, sts)
				break
			}
		}
	}
	return matches
}

// getServiceForStatefulSet finds the service that matches the StatefulSet
// TODO add handling for multiple services and headless services
func getServiceForStatefulSet(sts appsv1.StatefulSet, c kube.Cluster) (v1.Service, error) {
	ns := sts.Namespace

	services, err := c.Clientset.CoreV1().Services(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return v1.Service{}, fmt.Errorf("failed to list services: %w", err)
	}

	for _, svc := range services.Items {
		if reflect.DeepEqual(svc.Spec.Selector, sts.Spec.Selector.MatchLabels) {
			return svc, nil
		}
	}

	return v1.Service{}, fmt.Errorf("no matching service found for statefulset %s", sts.Name)
}

// setupTargetResources creates the service and StatefulSet in the target cluster
func setupTargetResources(ctx *mongo.MigrationContext, c kube.Clusters) error {
	service := ctx.Service
	serviceInterface := interface{}(service)
	serviceInterface = kube.CleanResourceForCreation(serviceInterface)
	service = *serviceInterface.(*v1.Service)

	statefulSet := ctx.StatefulSet
	statefulSetInterface := interface{}(statefulSet)
	statefulSetInterface = kube.CleanResourceForCreation(statefulSetInterface)
	statefulSet = *statefulSetInterface.(*appsv1.StatefulSet)

	if err := CreateResourceIfNotExists(c.Target, kube.Service, service.Namespace, &service); err != nil {
		return fmt.Errorf("failed to create service %s in target cluster: %w", service.Name, err)
	}

	if err := CreateResourceIfNotExists(c.Target, kube.StatefulSet, statefulSet.Namespace, &statefulSet); err != nil {
		return fmt.Errorf("failed to create StatefulSet %s in target cluster: %w", statefulSet.Name, err)
	}

	return nil
}

// CreateResourceIfNotExists creates a resource if it doesn't already exist
func CreateResourceIfNotExists(cluster kube.Cluster, resourceType kube.ResourceType, namespace string, resource interface{}) error {
	err := cluster.CreateResource(resourceType, namespace, resource)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) || strings.Contains(err.Error(), "already exists") {
			return nil // Resource already exists, this is fine
		}
		return err
	}
	return nil
}

// UpdateMongoHosts updates MongoDB host strings based on networking configuration
func UpdateMongoHosts(hosts []string, resources migration.Resources, service v1.Service, c kube.Cluster) []string {
	updatedHosts := make([]string, 0)

	if isHeadlessService(service) {
		for _, host := range hosts {
			podName, serviceName, namespace, err := extractMetadataFromDNSName(host)
			exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to extract metadata from DNS name %s", host))
			var updatedHost string
			if resources.GetNetworkingTool() == prompt.NetworkingToolSubmariner {
				updatedHost = resources.GetHeadlessDNSName(podName, serviceName, namespace, c.Name) + ":" + mongoPort
			} else if resources.GetNetworkingTool() == prompt.NetworkingToolSkupper {
				updatedHost = fmt.Sprintf("%s.%s-%s.%s.svc.cluster.local:27017", podName, serviceName, c.Name, namespace)
			} else if resources.GetNetworkingTool() == prompt.NetworkingToolLinkerd {
				updatedHost = fmt.Sprintf("%s.%s-%s.%s.svc.cluster.local:27017", podName, serviceName, c.Name, namespace)
			}

			updatedHosts = append(updatedHosts, updatedHost)
		}
	} else {
		for _, host := range hosts {
			_, serviceName, namespace, err := extractMetadataFromDNSName(host)
			exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to extract metadata from DNS name %s", host))
			updatedHost := resources.GetDNSName(serviceName, namespace)
			updatedHosts = append(updatedHosts, updatedHost)
		}
	}

	return updatedHosts
}

// isHeadlessService checks if a service is headless
func isHeadlessService(service v1.Service) bool {
	return service.Spec.ClusterIP == v1.ClusterIPNone
}

// extractMetadataFromDNSName extracts pod name, service name, and namespace from DNS name
func extractMetadataFromDNSName(dnsName string) (string, string, string, error) {
	parts := strings.Split(dnsName, ".")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("invalid DNS name format: %s", dnsName)
	}
	return parts[0], parts[1], parts[2], nil
}
