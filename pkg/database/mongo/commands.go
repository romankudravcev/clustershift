package mongo

import (
	"bytes"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"encoding/json"
	"fmt"
	"strings"
)

// execMongoCommand executes a MongoDB command and handles common error cases
func execMongoCommand(c kube.Cluster, podName, namespace, command string) error {
	var out, errOut bytes.Buffer

	cmd := []string{mongoshCommand, "--eval", command}
	err := c.ExecIntoPod(namespace, podName, cmd, &out, &errOut)
	if err != nil {
		return fmt.Errorf("failed to execute MongoDB command: %w, stderr: %s", err, errOut.String())
	}
	if errOut.Len() > 0 {
		return fmt.Errorf("mongosh error: %s", errOut.String())
	}
	return nil
}

// getMongoHosts retrieves all MongoDB replica set member hosts
func getMongoHosts(c kube.Cluster, podName, namespace string) ([]string, error) {
	command := []string{mongoshCommand, "--eval", "JSON.stringify(rs.conf())"}
	var out, errOut bytes.Buffer

	err := c.ExecIntoPod(namespace, podName, command, &out, &errOut)
	if err != nil {
		return nil, fmt.Errorf("failed to exec into pod: %w", err)
	}
	if errOut.Len() > 0 {
		return nil, fmt.Errorf("mongosh error: %s", errOut.String())
	}

	var conf struct {
		Members []struct {
			Host string `json:"host"`
		} `json:"members"`
	}
	if err := json.Unmarshal(out.Bytes(), &conf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mongosh output: %w", err)
	}

	hosts := make([]string, len(conf.Members))
	for i, m := range conf.Members {
		hosts[i] = m.Host
	}
	return hosts, nil
}

// getPrimaryMongoHost finds the primary MongoDB host in the replica set
func getPrimaryMongoHost(c kube.Cluster, podName, namespace string) (string, error) {
	command := []string{mongoshCommand, "--eval", "JSON.stringify(rs.status())"}
	var out, errOut bytes.Buffer

	err := c.ExecIntoPod(namespace, podName, command, &out, &errOut)
	if err != nil {
		return "", fmt.Errorf("failed to exec into pod: %w", err)
	}
	if errOut.Len() > 0 {
		return "", fmt.Errorf("mongosh error: %s", errOut.String())
	}

	var status struct {
		Members []struct {
			Name     string `json:"name"`
			StateStr string `json:"stateStr"`
		} `json:"members"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		return "", fmt.Errorf("failed to unmarshal rs.status output: %w", err)
	}

	for _, member := range status.Members {
		if member.StateStr == primaryState {
			//return string before first dot
			parts := strings.Split(member.Name, ".")
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("no PRIMARY member found in replica set")
}

// isMongoMemberSecondary checks if a MongoDB member is in SECONDARY state
func isMongoMemberSecondary(c kube.Cluster, podName, namespace, host string) (bool, error) {
	var out, errOut bytes.Buffer
	command := []string{mongoshCommand, "--eval", "JSON.stringify(rs.status())"}

	err := c.ExecIntoPod(namespace, podName, command, &out, &errOut)
	if err != nil {
		return false, fmt.Errorf("failed to exec into pod: %w, stderr: %s", err, errOut.String())
	}
	if errOut.Len() > 0 {
		return false, fmt.Errorf("mongosh error: %s", errOut.String())
	}

	var status struct {
		Members []struct {
			Name     string `json:"name"`
			StateStr string `json:"stateStr"`
		} `json:"members"`
	}
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		return false, fmt.Errorf("failed to unmarshal rs.status output: %w", err)
	}

	for _, member := range status.Members {
		logger.Debug(fmt.Sprintf("Checking member: %s, state: %s. Should be %s, %s", member.Name, member.StateStr, host, secondaryState))
		if member.Name == host && member.StateStr == secondaryState {
			return true, nil
		}
	}
	return false, nil
}
