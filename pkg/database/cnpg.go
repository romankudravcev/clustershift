package cnpg

import (
	"clustershift/internal/cli"
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/pkg/submariner"
	"encoding/json"
	"fmt"
	"time"

	apiv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func InstallOperator(c kube.Cluster, logger *cli.Logger) {
	l := logger.Log("Installing cloud native-pg operator")
	exit.OnErrorWithMessage(c.CreateResourcesFromURL(constants.CNPGOperatorURL), "failed installing cloud native-pg operator")
	l.Success("Installed cloud native-pg operator")
}

func ConvertToCluster(data map[string]interface{}) (*apiv1.Cluster, error) {
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

func ConvertFromCluster(cluster *apiv1.Cluster) (map[string]interface{}, error) {
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

func addRWServiceToYaml(c kube.Cluster, resources []map[string]interface{}) error {
	for _, resource := range resources {
		name := resource["metadata"].(map[string]interface{})["name"].(string)
		namespace := resource["metadata"].(map[string]interface{})["namespace"].(string)
		dns := fmt.Sprintf("origin.%s-rw.%s.svc.clusterset.local", name, namespace)

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

func AddClustersetDNS(c kube.Cluster, logger *cli.Logger) {
	l := logger.Log("Adding submariner clusterset DNS")

	// Fetch all cnpg clusters
	l1 := l.Log("fetching cnpg clusters")
	resources, err := c.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")
	l1.Success("Fetched clusters")

	// Add clusterset DNS to each cluster
	l1 = l.Log("Updating cluster resources")
	err = addRWServiceToYaml(c, resources)
	exit.OnErrorWithMessage(err, "Error updating cluster resources")
	l1.Success("Updated cluster resources")

	l.Success("Added submariner clusterset DNS")
}

func createReplicaCluster(originCluster *apiv1.Cluster) (*apiv1.Cluster, error) {
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
				"host":    "origin." + originCluster.Name + "-rw." + originCluster.Namespace + ".svc.clusterset.local",
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

func CreateReplicaClusters(c kube.Clusters, logger *cli.Logger) {
	l := logger.Log("Creating replica cluster")

	// Fetch cnpg clusters from origin
	l1 := l.Log("Fetching origin cnpg clusters")
	resources, err := c.Origin.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")
	l1.Success("Fetched origin clusters")

	l1 = l.Log("Creating replica cluster from origin")
	for _, resource := range resources {
		// Convert origin cluster to API object
		originCluster, err := ConvertToCluster(resource)
		exit.OnErrorWithMessage(err, "Error converting origin cluster")

		// Create replica cluster from origin
		replicaCluster, err := createReplicaCluster(originCluster)
		exit.OnErrorWithMessage(err, "Error creating replica cluster")

		cli.LogToFile(fmt.Sprintf("%v", replicaCluster))
		// Convert replica cluster to data
		replicaClusterData, err := ConvertFromCluster(replicaCluster)
		exit.OnErrorWithMessage(err, "Error converting replica cluster")

		// Create replica cluster
		err = c.Target.CreateCustomResource(originCluster.Namespace, replicaClusterData, l1)
		exit.OnErrorWithMessage(err, "Error applying replica cluster")

		// Wait for replica cluster to be ready
		err = kube.WaitForCNPGClusterReady(c.Target.DynamicClientset, originCluster.Name, originCluster.Namespace, 1*time.Hour)
		exit.OnErrorWithMessage(err, "Timeout while waiting for replica cluster bootstrap")
	}
	l1.Success("Created replica clusters")
}

func ExportRWServices(c kube.Cluster, logger *cli.Logger) {
	l := logger.Log("Exporting cnpg rw services")

	// Fetch all cnpg clusters
	l1 := l.Log("fetching cnpg clusters")
	resources, err := c.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")
	l1.Success("Fetched clusters")

	// Export services
	for _, resource := range resources {
		clusterName := resource["metadata"].(map[string]interface{})["name"].(string)
		namespace := resource["metadata"].(map[string]interface{})["namespace"].(string)
		serviceName := fmt.Sprintf("%s-rw", clusterName)
		submariner.Export(c, namespace, serviceName, "", l)
	}

	l.Success("Service export successful")
}

func DemoteOriginCluster(c kube.Cluster, logger *cli.Logger) {
	l := logger.Log("Demote cnpg clusters")

	// Fetch all cnpg clusters
	l1 := l.Log("fetching cnpg clusters")
	resources, err := c.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")
	l1.Success("Fetched clusters")

	for _, resource := range resources {
		cluster, err := ConvertToCluster(resource)
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
		updatedObj, err := ConvertFromCluster(cluster)
		exit.OnErrorWithMessage(err, "Error converting updated cluster to unstructured")

		// Update the resource
		err = c.UpdateCustomResource(cluster.Namespace, updatedObj)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Error updating cluster %s in namespace %s", cluster.Name, cluster.Namespace))

		l1.Success(fmt.Sprintf("Successfully updated cluster %s in namespace %s", cluster.Name, cluster.Namespace))
	}
	l.Success("Completed demoting clusters")
}

func DisableReplication(c kube.Cluster, logger *cli.Logger) {
	l := logger.Log("Demote cnpg clusters")

	// Fetch all cnpg clusters
	l1 := l.Log("fetching cnpg clusters")
	resources, err := c.FetchCustomResources(
		"postgresql.cnpg.io",
		"v1",
		"clusters",
	)
	exit.OnErrorWithMessage(err, "Error fetching custom resources")
	l1.Success("Fetched clusters")

	for _, resource := range resources {
		cluster, err := ConvertToCluster(resource)
		exit.OnErrorWithMessage(err, "Error converting origin cluster")

		enabled := false
		cluster.Spec.ReplicaCluster.Enabled = &enabled

		// Convert the updated cluster back to unstructured
		updatedObj, err := ConvertFromCluster(cluster)
		exit.OnErrorWithMessage(err, "Error converting updated cluster to unstructured")

		// Update the resource
		err = c.UpdateCustomResource(cluster.Namespace, updatedObj)
		if err != nil {
			exit.OnErrorWithMessage(err, fmt.Sprintf("Error updating cluster %s in namespace %s", cluster.Name, cluster.Namespace))
		}

		l1.Success(fmt.Sprintf("Successfully updated cluster %s in namespace %s", cluster.Name, cluster.Namespace))
	}
	l.Success("Completed demoting clusters")
}
