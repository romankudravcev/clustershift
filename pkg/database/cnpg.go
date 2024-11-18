package cnpg

import (
	"clustershift/internal/cli"
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"encoding/json"

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
