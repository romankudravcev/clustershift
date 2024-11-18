package cnpg

import (
	"clustershift/internal/cli"
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"encoding/json"
	"fmt"

	apiv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
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
