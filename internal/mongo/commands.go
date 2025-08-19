package mongo

import (
	"clustershift/internal/exit"
	"clustershift/internal/logger"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// TODO: IMPLEMENT USER AND PASSWORD FETCHING FROM ENVIRONMENT VARIABLES OR CONFIGURATION
const (
	username = "admin"
	password = "admin123"
	//username = "my-user"
	//password = "password"
)

// execMongoCommand executes a MongoDB command using a client pod
func execMongoCommand(client *Client, mongoHost, command string) (string, error) {
	cmd := []string{
		mongoshCommand,
		fmt.Sprintf("mongodb://%s:%s@%s/admin?authSource=admin", username, password, mongoHost),
		"--eval", command,
	}

	return client.ExecMongoCommand(cmd)
}

func execMongoCommandWithoutUser(client *Client, mongoHost, command string) (string, error) {
	cmd := []string{
		mongoshCommand,
		"--eval", command,
	}

	return client.ExecMongoCommand(cmd)
}

// execMongoScript executes a MongoDB script using a client pod
func execMongoScript(client *Client, mongoHost, script string) (string, error) {
	escapedScript := strings.ReplaceAll(script, "\n", " ")
	escapedScript = strings.ReplaceAll(escapedScript, "\t", " ")
	// Remove multiple spaces
	escapedScript = regexp.MustCompile(`\s+`).ReplaceAllString(escapedScript, " ")
	escapedScript = strings.TrimSpace(escapedScript)

	return execMongoCommand(client, mongoHost, escapedScript)
}

// GetMongoHosts retrieves all MongoDB replica set member hosts using client pod
func GetMongoHosts(client *Client, mongoHost string) ([]string, error) {
	cmd := []string{
		mongoshCommand,
		fmt.Sprintf("mongodb://%s:%s@%s/admin?authSource=admin", username, password, mongoHost),
		"--eval", "JSON.stringify(rs.conf())",
	}

	output, err := client.ExecMongoCommand(cmd)
	exit.OnErrorWithMessage(err, "Failed to get MongoDB hosts")

	// Extract JSON from the output by finding the first '{' and last '}'
	jsonStart := strings.Index(output, "{")
	if jsonStart == -1 {
		return nil, fmt.Errorf("no JSON found in mongosh output: %s", output)
	}

	jsonEnd := strings.LastIndex(output, "}")
	if jsonEnd == -1 || jsonEnd < jsonStart {
		return nil, fmt.Errorf("invalid JSON format in mongosh output: %s", output)
	}

	jsonStr := output[jsonStart : jsonEnd+1]

	var conf struct {
		Members []struct {
			Host string `json:"host"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &conf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mongosh output: %w, json: %s", err, jsonStr)
	}

	hosts := make([]string, len(conf.Members))
	for i, m := range conf.Members {
		hosts[i] = m.Host
	}
	return hosts, nil
}

// GetMongoHostsAuthenticated retrieves all MongoDB replica set member hosts using client pod
func GetMongoHostsAuthenticated(client *Client, mongoHost string) ([]string, error) {
	cmd := []string{
		mongoshCommand,
		fmt.Sprintf("mongodb://%s:%s@%s/admin?authSource=admin", username, password, mongoHost),
		"--eval", "JSON.stringify(rs.conf())",
	}

	output, err := client.ExecMongoCommand(cmd)
	exit.OnErrorWithMessage(err, "Failed to get MongoDB hosts")

	// Extract JSON from the output by finding the first '{' and last '}'
	jsonStart := strings.Index(output, "{")
	if jsonStart == -1 {
		return nil, fmt.Errorf("no JSON found in mongosh output: %s", output)
	}

	jsonEnd := strings.LastIndex(output, "}")
	if jsonEnd == -1 || jsonEnd < jsonStart {
		return nil, fmt.Errorf("invalid JSON format in mongosh output: %s", output)
	}

	jsonStr := output[jsonStart : jsonEnd+1]

	var conf struct {
		Members []struct {
			Host string `json:"host"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &conf); err != nil {
		return nil, fmt.Errorf("failed to unmarshal mongosh output: %w, json: %s", err, jsonStr)
	}

	hosts := make([]string, len(conf.Members))
	for i, m := range conf.Members {
		hosts[i] = m.Host
	}
	return hosts, nil
}

// GetPrimaryMongoHost finds the primary MongoDB host using client pod
func GetPrimaryMongoHost(client *Client, mongoHost string) (string, error) {
	cmd := []string{
		mongoshCommand,
		fmt.Sprintf("mongodb://%s:%s@%s:27017/admin?authSource=admin", username, password, mongoHost),
		"--eval", "JSON.stringify(rs.status())",
	}

	output, err := client.ExecMongoCommand(cmd)
	exit.OnErrorWithMessage(err, "Failed to get primary MongoDB host")

	jsonStart := strings.Index(output, "{")
	if jsonStart == -1 {
		return "", fmt.Errorf("no JSON found in mongosh output: %s", output)
	}

	jsonEnd := strings.LastIndex(output, "}")
	if jsonEnd == -1 || jsonEnd < jsonStart {
		return "", fmt.Errorf("invalid JSON format in mongosh output: %s", output)
	}

	jsonStr := output[jsonStart : jsonEnd+1]

	var status struct {
		Members []struct {
			Name     string `json:"name"`
			StateStr string `json:"stateStr"`
		} `json:"members"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &status); err != nil {
		return "", fmt.Errorf("failed to unmarshal rs.status output: %w", err)
	}

	for _, member := range status.Members {
		if member.StateStr == primaryState {
			// Return string before first dot
			parts := strings.Split(member.Name, ":")
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("no PRIMARY member found in replica set")
}

// isMongoMemberSecondary checks if a MongoDB member is in SECONDARY state using client pod
func isMongoMemberSecondary(client *Client, mongoHost, targetHost string) (bool, error) {
	cmd := []string{
		mongoshCommand,
		fmt.Sprintf("mongodb://%s:%s@%s:27017/admin?authSource=admin", username, password, mongoHost),
		"--eval", "JSON.stringify(rs.status())",
		"--quiet",
	}

	output, err := client.ExecMongoCommand(cmd)
	exit.OnErrorWithMessage(err, "Failed to get MongoDB status")

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
func CreateSyncUser(client *Client, mongoHost string) error {
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
	_, err := execMongoScript(client, mongoHost, script)
	return err
}

func CreateRootUser(client *Client, mongoHost string, username, password string) error {
	script := fmt.Sprintf(`
db = db.getSiblingDB("admin");
db.createUser({
  user: '%s',
  pwd: '%s',
  roles: [ { role: 'root', db: 'admin' } ]
});`, username, password)
	_, err := execMongoCommandWithoutUser(client, mongoHost, script)
	if err == nil {
		logger.Info("Root user created successfully")
	} else {
		logger.Error("Failed to create root user: %v", err)
	}
	return err
}

func CreateTestUser(client *Client, mongoHost string) error {
	script := `
db = db.getSiblingDB("testdb");
db.createUser({
  user: "testdb_user",
  pwd: "password",
  roles: [
    { role: "readWrite", db: "testdb" },
    { role: "dbAdmin", db: "testdb" },
    { role: "userAdmin", db: "testdb" }
  ]
});
`
	_, err := execMongoScript(client, mongoHost, script)
	return err
}
