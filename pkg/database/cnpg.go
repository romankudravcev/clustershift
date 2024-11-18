package cnpg

import (
	"clustershift/internal/cli"
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"encoding/json"
	"fmt"

	apiv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func InstallOperator(c kube.Cluster, logger *cli.Logger) {
	l := logger.Log("Installing cloudnative-pg operator")
	err := c.CreateResourcesFromURL(constants.CNPGOperatorURL)
	exit.OnErrorWithMessage(l.Fail("failed installing cloudnative-pg operator", err))
	l.Success("Installed cloudnative-pg operator")
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
	exit.OnErrorWithMessage(l1.Fail("Error fetching custom resources", err))
	l1.Success("Fetched clusters")

	// Add clusterset DNS to each cluster
	l1 = l.Log("Updating cluster resources")
	err = addRWServiceToYaml(c, resources)
	exit.OnErrorWithMessage(l1.Fail("Error updating cluster resources", err))
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
	exit.OnErrorWithMessage(l1.Fail("Error fetching custom resources", err))
	l1.Success("Fetched origin clusters")

	l1 = l.Log("Creating replica cluster from origin")
	for _, resource := range resources {
		// Convert origin cluster to API object
		originCluster, err := convertToCluster(resource)
		exit.OnErrorWithMessage(l1.Fail("Error converting origin cluster", err))

		// Create replica cluster from origin
		replicaCluster, err := createReplicaCluster(originCluster)
		exit.OnErrorWithMessage(l1.Fail("Error creating replica cluster", err))

		cli.LogToFile(fmt.Sprintf("%v", replicaCluster))
		// Convert replica cluster to data
		replicaClusterData, err := convertFromCluster(replicaCluster)
		exit.OnErrorWithMessage(l1.Fail("Error converting replica cluster", err))

		// Create replica cluster
		err = c.Target.CreateCustomResource(originCluster.Namespace, replicaClusterData, l1)
		exit.OnErrorWithMessage(l1.Fail("Error applying replica cluster", err))
	}
	l1.Success("Created replica clusters")
}
