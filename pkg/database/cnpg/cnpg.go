package cnpg

import (
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"clustershift/internal/migration"
	"clustershift/internal/prompt"
	"clustershift/pkg/skupper"
	"encoding/json"
	"fmt"
	"time"

	apiv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Migrate(clusters kube.Clusters, resources migration.Resources) {
	logger.Info("Scanning for existing cnpg databases")
	exists := scanExistingDatabases(clusters.Origin)

	if !exists {
		logger.Info("No existing cnpg databases found, skipping migration")
		return
	}

	logger.Info("Migrate cnpg databases")
	installOperator(clusters.Target)
	err := kube.WaitForPodsReady(clusters.Target, constants.CNPGLabelSelector, constants.CNPGNamespace, 90*time.Second)
	exit.OnErrorWithMessage(err, "Failed to wait for CNPG pods to be ready")

	addClustersetDNS(clusters.Origin, resources)
	exportRWServices(clusters, clusters.Origin, resources)
	createReplicaClusters(clusters, resources)
}
func installOperator(c kube.Cluster) {
	logger.Info("Installing cloud native-pg operator")
	exit.OnErrorWithMessage(c.CreateResourcesFromURL(constants.CNPGOperatorURL, "cnpg-system"), "failed installing cloud native-pg operator")
}

func convertToCluster(data map[string]interface{}) (*apiv1.Cluster, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	cluster := &apiv1.Cluster{}
	if err := json.Unmarshal(jsonData, cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

func convertFromCluster(cluster *apiv1.Cluster) (map[string]interface{}, error) {
	// Marshal cluster to JSON
	jsonData, err := json.Marshal(cluster)
	if err != nil {
		return nil, err
	}

	// Unmarshal into single map
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func addRWServiceToYaml(c kube.Cluster, resources []map[string]interface{}, migrationResources migration.Resources) error {
	for _, resource := range resources {
		name := resource["metadata"].(map[string]interface{})["name"].(string)
		namespace := resource["metadata"].(map[string]interface{})["namespace"].(string)

		if migrationResources.GetNetworkingTool() == prompt.NetworkingToolSkupper {
			name = name + "-rw-" + c.Name
		}

		dns := migrationResources.GetDNSName(name, namespace)

		spec := resource["spec"].(map[string]interface{})

		if _, exists := spec["certificates"]; !exists {
			spec["certificates"] = make(map[string]interface{})
		}

		certificates := spec["certificates"].(map[string]interface{})

		var dnsNames []string
		if existing, exists := certificates["serverAltDNSNames"]; exists {
			for _, name := range existing.([]interface{}) {
				dnsNames = append(dnsNames, name.(string))
			}
		}

		found := false
		for _, name := range dnsNames {
			if name == dns {
				found = true
				break
			}
		}
		if !found {
			dnsNames = append(dnsNames, dns)
		}

		certificates["serverAltDNSNames"] = dnsNames

		err := c.UpdateCustomResource(namespace, resource)
		if err != nil {
			return err
		}
	}
	return nil
}

func addClustersetDNS(c kube.Cluster, migrationResources migration.Resources) {
	logger.Info("Adding submariner clusterset DNS")

	// Fetch all cnpg clusters
	logger.Info("fetching cnpg clusters")
	resources, err := c.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")

	// Add clusterset DNS to each cluster
	logger.Info("Updating cluster resources")
	err = addRWServiceToYaml(c, resources, migrationResources)
	exit.OnErrorWithMessage(err, "Error updating cluster resources")
}

func createReplicaCluster(c kube.Cluster, originCluster *apiv1.Cluster, migrationResources migration.Resources) (*apiv1.Cluster, error) {
	// Create a deep copy of the origin cluster
	replicaCluster := originCluster.DeepCopy()

	// Reset ObjectMeta but keep namespace and name
	replicaCluster.ObjectMeta = metav1.ObjectMeta{
		Name:      originCluster.Name,
		Namespace: originCluster.Namespace,
	}

	// Clear status
	replicaCluster.Status = apiv1.ClusterStatus{}

	// Configure bootstrap for recovery
	replicaCluster.Spec.Bootstrap = &apiv1.BootstrapConfiguration{
		PgBaseBackup: &apiv1.BootstrapPgBaseBackup{
			Source: originCluster.Name,
		},
	}

	// Configure replica for replication
	enabled := true
	replicaCluster.Spec.ReplicaCluster = &apiv1.ReplicaClusterConfiguration{
		Enabled: &enabled,
		Source:  originCluster.Name,
	}

	// Configure external cluster for replication
	replicaCluster.Spec.ExternalClusters = []apiv1.ExternalCluster{
		{
			Name: originCluster.Name,
			ConnectionParameters: map[string]string{
				"host":    migrationResources.GetCNPGHostname(c.Name, originCluster.Name, originCluster.Namespace),
				"user":    "streaming_replica",
				"dbname":  "postgres",
				"sslmode": "verify-full",
			},

			SSLCert: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: originCluster.Name + "-replication",
				},
				Key: "tls.crt",
			},
			SSLKey: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: originCluster.Name + "-replication",
				},
				Key: "tls.key",
			},
			SSLRootCert: &v1.SecretKeySelector{
				LocalObjectReference: v1.LocalObjectReference{
					Name: originCluster.Name + "-ca",
				},
				Key: "ca.crt",
			},
		},
	}

	return replicaCluster, nil
}

func createReplicaClusters(c kube.Clusters, migrationResources migration.Resources) {
	logger.Info("Creating replica cluster")

	// Fetch cnpg clusters from origin
	logger.Info("Fetching origin cnpg clusters")
	resources, err := c.Origin.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")
	logger.Info("Fetched origin clusters")

	logger.Info("Creating replica cluster from origin")
	for _, resource := range resources {
		// Convert origin cluster to API object
		originCluster, err := convertToCluster(resource)
		exit.OnErrorWithMessage(err, "Error converting origin cluster")

		// Create replica cluster from origin
		replicaCluster, err := createReplicaCluster(c.Origin, originCluster, migrationResources)
		exit.OnErrorWithMessage(err, "Error creating replica cluster")

		logger.Debug(fmt.Sprintf("%v", replicaCluster))
		// Convert replica cluster to data
		replicaClusterData, err := convertFromCluster(replicaCluster)
		exit.OnErrorWithMessage(err, "Error converting replica cluster")

		// Create replica cluster
		err = c.Target.CreateCustomResource(originCluster.Namespace, replicaClusterData)
		exit.OnErrorWithMessage(err, "Error applying replica cluster")

		// Wait for replica cluster to be ready
		err = kube.WaitForCNPGClusterReady(c.Target.DynamicClientset, originCluster.Name, originCluster.Namespace, 1*time.Hour)
		exit.OnErrorWithMessage(err, "Timeout while waiting for replica cluster bootstrap")
	}
	logger.Info("Created replica clusters")
}

func exportRWServices(clusters kube.Clusters, c kube.Cluster, migrationResources migration.Resources) {
	logger.Info("Exporting cnpg rw services")

	// Fetch all cnpg clusters
	logger.Info("fetching cnpg clusters")
	resources, err := c.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")

	// Export services
	for _, resource := range resources {
		clusterName := resource["metadata"].(map[string]interface{})["name"].(string)
		namespace := resource["metadata"].(map[string]interface{})["namespace"].(string)
		serviceName := fmt.Sprintf("%s-rw", clusterName)

		if migrationResources.GetNetworkingTool() == prompt.NetworkingToolSkupper {
			skupper.CreateSiteConnection(clusters, namespace)
		}
		migrationResources.ExportService(c, namespace, serviceName)
	}

}

func DemoteOriginCluster(c kube.Cluster) {
	logger.Info("Demote cnpg clusters")

	// Fetch all cnpg clusters
	logger.Info("fetching cnpg clusters")
	resources, err := c.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")
	logger.Info("Fetched clusters")

	for _, resource := range resources {
		cluster, err := convertToCluster(resource)
		exit.OnErrorWithMessage(err, "Error converting origin cluster")

		// Update cluster spec
		cluster.Spec.ExternalClusters = []apiv1.ExternalCluster{
			{
				Name: cluster.Name + "-new",
				ConnectionParameters: map[string]string{
					"host":    "target." + cluster.Name + "-rw." + cluster.Namespace + ".svc.clusterset.local",
					"user":    "streaming_replica",
					"dbname":  "postgres",
					"sslmode": "verify-full",
				},
				SSLCert: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: cluster.Name + "-replication",
					},
					Key: "tls.crt",
				},
				SSLKey: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: cluster.Name + "-replication",
					},
					Key: "tls.key",
				},
				SSLRootCert: &v1.SecretKeySelector{
					LocalObjectReference: v1.LocalObjectReference{
						Name: cluster.Name + "-ca",
					},
					Key: "ca.crt",
				},
			},
		}

		cluster.Spec.ReplicaCluster = &apiv1.ReplicaClusterConfiguration{
			Primary: cluster.Name + "-new",
			Source:  cluster.Name + "-new",
		}

		// Convert the updated cluster back to unstructured
		updatedObj, err := convertFromCluster(cluster)
		exit.OnErrorWithMessage(err, "Error converting updated cluster to unstructured")

		// Update the resource
		err = c.UpdateCustomResource(cluster.Namespace, updatedObj)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Error updating cluster %s in namespace %s", cluster.Name, cluster.Namespace))

		logger.Info(fmt.Sprintf("Successfully updated cluster %s in namespace %s", cluster.Name, cluster.Namespace))
	}
	logger.Info("Completed demoting clusters")
}

func DisableReplication(c kube.Cluster) {
	logger.Info("Demote cnpg clusters")

	// Fetch all cnpg clusters
	logger.Info("fetching cnpg clusters")
	resources, err := c.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")
	logger.Info("Fetched clusters")

	for _, resource := range resources {
		cluster, err := convertToCluster(resource)
		exit.OnErrorWithMessage(err, "Error converting origin cluster")

		enabled := false
		cluster.Spec.ReplicaCluster.Enabled = &enabled

		// Convert the updated cluster back to unstructured
		updatedObj, err := convertFromCluster(cluster)
		exit.OnErrorWithMessage(err, "Error converting updated cluster to unstructured")

		// Update the resource
		err = c.UpdateCustomResource(cluster.Namespace, updatedObj)
		if err != nil {
			exit.OnErrorWithMessage(err, fmt.Sprintf("Error updating cluster %s in namespace %s", cluster.Name, cluster.Namespace))
		}

		logger.Info(fmt.Sprintf("Successfully updated cluster %s in namespace %s", cluster.Name, cluster.Namespace))
	}
	logger.Info("Completed demoting clusters")
}

func scanExistingDatabases(c kube.Cluster) bool {
	resources, err := c.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")

	if len(resources) == 0 {
		return false
	}
	return true
}
