package statefulset

import (
	"bytes"
	"clustershift/internal/logger"
	"encoding/json"
	"fmt"
	"strings"
)

// execMongoCommand executes a MongoDB command using a client pod
func execMongoCommand(client *MongoClient, mongoHost, command string) error {
	return client.ExecMongoCommand(mongoHost, command)
}

// execMongoScript executes a MongoDB script using a client pod
func execMongoScript(client *MongoClient, mongoHost, script string) error {
	return client.ExecMongoScript(mongoHost, script)
}

// GetMongoHosts retrieves all MongoDB replica set member hosts using client pod
func GetMongoHosts(client *MongoClient, mongoHost string) ([]string, error) {
	return client.GetMongoHostsFromClient(mongoHost)
}

// GetMongoHostsAuthenticated retrieves all MongoDB replica set member hosts using client pod
func GetMongoHostsAuthenticated(client *MongoClient, mongoHost string) ([]string, error) {
	return client.GetMongoHostsFromClient(mongoHost)
}

// GetPrimaryMongoHost finds the primary MongoDB host using client pod
func GetPrimaryMongoHost(client *MongoClient, mongoHost string) (string, error) {
	return client.GetPrimaryMongoHostFromClient(mongoHost)
}

// isMongoMemberSecondary checks if a MongoDB member is in SECONDARY state using client pod
func isMongoMemberSecondary(client *MongoClient, mongoHost, targetHost string) (bool, error) {
	if !client.isReady {
		return false, fmt.Errorf("MongoDB client pod is not ready")
	}

	var out, errOut bytes.Buffer

	cmd := []string{
		mongoshCommand,
		fmt.Sprintf("mongodb://my-user:password@%s:27017/admin?authSource=admin", mongoHost),
		"--eval", "JSON.stringify(rs.status())",
		"--quiet",
	}

	err := client.cluster.ExecIntoPod(client.namespace, client.podName, "mongosh-client", cmd, &out, &errOut)
	if err != nil {
		return false, fmt.Errorf("failed to exec into client pod: %w, stderr: %s", err, errOut.String())
	}
	if errOut.Len() > 0 {
		return false, fmt.Errorf("mongosh error: %s", errOut.String())
	}

	output := out.String()
	jsonStart := strings.Index(output, "{")
	if jsonStart == -1 {
		return false, fmt.Errorf("no JSON found in mongosh output: %s", output)
	}

	jsonEnd := strings.LastIndex(output, "}")
	if jsonEnd == -1 || jsonEnd < jsonStart {
		return false, fmt.Errorf("invalid JSON format in mongosh output: %s", output)
	}

	jsonStr := output[jsonStart : jsonEnd+1]

	var status struct {
		Members []struct {
			Name     string `json:"name"`
			StateStr string `json:"stateStr"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &status); err != nil {
		return false, fmt.Errorf("failed to unmarshal rs.status output: %w", err)
	}

	for _, member := range status.Members {
		logger.Debug(fmt.Sprintf("Checking member: %s, state: %s. Should be %s, %s", member.Name, member.StateStr, targetHost, secondaryState))
		if member.Name == targetHost && member.StateStr == secondaryState {
			return true, nil
		}
	}
	return false, nil
}

// CreateSyncUser creates a sync user using client pod
func CreateSyncUser(client *MongoClient, mongoHost string) error {
	script := `
db.createUser({
  user: "clusteradmin",
  pwd: "password1", 
  roles: [
    { role: "clusterAdmin", db: "admin" },
    { role: "readWriteAnyDatabase", db: "admin" },
    { role: "dbAdminAnyDatabase", db: "admin" },
    { role: "restore", db: "admin" },
    { role: "backup", db: "admin" },
    { role: "root", db: "admin" }
  ]
})
`
	return execMongoScript(client, mongoHost, script)
}
