package kubeconfig

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

func ProcessKubeconfig(kubeconfigPath string, clusterType string) {
	v := viper.New()
	v.SetConfigFile(kubeconfigPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		fmt.Println("Error reading input file:", err)
		return
	}

	newClusterName, newUserName, newContextName := getClusterDetails(clusterType)

	updateClusters(v, newClusterName)
	updateContexts(v, newClusterName, newUserName, newContextName)
	updateUsers(v, newUserName)
	v.Set("current-context", newContextName)

	outputPath := getOutputPath(clusterType)
	if err := os.MkdirAll("tmp", os.ModePerm); err != nil {
		fmt.Println("Error creating output directory:", err)
		return
	}

	if err := v.WriteConfigAs(outputPath); err != nil {
		fmt.Println("Error writing output file:", err)
		return
	}

	fmt.Printf("Kubeconfig transformation successful! Output written to %s\n", outputPath)
}

func getClusterDetails(clusterType string) (string, string, string) {
	switch clusterType {
	case "origin":
		return "cluster-origin", "user-origin", "context-origin"
	case "target":
		return "cluster-target", "user-target", "context-target"
	default:
		return "", "", ""
	}
}

func updateClusters(v *viper.Viper, newClusterName string) {
	if clusters, ok := v.Get("clusters").([]interface{}); ok {
		for i, cluster := range clusters {
			if clusterMap, ok := cluster.(map[string]interface{}); ok {
				clusterMap["name"] = newClusterName
				clusters[i] = clusterMap
				break
			}
		}
		v.Set("clusters", clusters)
	}
}

func updateContexts(v *viper.Viper, newClusterName, newUserName, newContextName string) {
	if contexts, ok := v.Get("contexts").([]interface{}); ok {
		for i, context := range contexts {
			if contextMap, ok := context.(map[string]interface{}); ok {
				if ctx, ok := contextMap["context"].(map[string]interface{}); ok {
					ctx["cluster"] = newClusterName
					ctx["user"] = newUserName
				}
				contextMap["name"] = newContextName
				contexts[i] = contextMap
				break
			}
		}
		v.Set("contexts", contexts)
	}
}

func updateUsers(v *viper.Viper, newUserName string) {
	if users, ok := v.Get("users").([]interface{}); ok {
		for i, user := range users {
			if userMap, ok := user.(map[string]interface{}); ok {
				userMap["name"] = newUserName
				users[i] = userMap
				break
			}
		}
		v.Set("users", users)
	}
}

func getOutputPath(clusterType string) string {
	switch clusterType {
	case "origin":
		return "tmp/origin_kubeconfig.yaml"
	case "target":
		return "tmp/target_kubeconfig.yaml"
	default:
		return ""
	}
}
