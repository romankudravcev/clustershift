package statefulset

import (
	"bytes"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MongoClient manages a MongoDB client pod for executing commands
type MongoClient struct {
	cluster   kube.Cluster
	namespace string
	podName   string
	isReady   bool
}

// NewMongoClient creates a new MongoDB client instance
func NewMongoClient(cluster kube.Cluster, namespace string) *MongoClient {
	return &MongoClient{
		cluster:   cluster,
		namespace: namespace,
		podName:   mongoClientPodName,
		isReady:   false,
	}
}

// CreateClientPod creates a MongoDB client pod for executing commands
func (mc *MongoClient) CreateClientPod() error {
	logger.Info("Creating MongoDB client pod...")

	// Create the client pod using your existing infrastructure
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc.podName,
			Namespace: mc.namespace,
			Labels: map[string]string{
				"app":  "mongosh-client",
				"role": "database-client",
			},
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    "mongosh-client",
					Image:   mongoClientImage,
					Command: []string{"sleep", "3600"}, // Keep pod alive for 1 hour
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("128Mi"),
							v1.ResourceCPU:    resource.MustParse("100m"),
						},
						Limits: v1.ResourceList{
							v1.ResourceMemory: resource.MustParse("256Mi"),
							v1.ResourceCPU:    resource.MustParse("200m"),
						},
					},
				},
			},
		},
	}

	// Use your existing CreateResource method
	err := mc.cluster.CreateResource(kube.Pod, mc.namespace, pod)
	if err != nil {
		return fmt.Errorf("failed to create MongoDB client pod: %w", err)
	}

	// Wait for pod to be ready
	logger.Info("Waiting for MongoDB client pod to be ready...")
	err = mc.waitForPodReady()
	if err != nil {
		return fmt.Errorf("MongoDB client pod failed to become ready: %w", err)
	}

	mc.isReady = true
	logger.Info("MongoDB client pod is ready")
	return nil
}

// DeleteClientPod deletes the MongoDB client pod
func (mc *MongoClient) DeleteClientPod() error {
	if !mc.isReady {
		return nil
	}

	logger.Info("Deleting MongoDB client pod...")

	// Use your existing DeleteResource method
	err := mc.cluster.DeleteResource(kube.Pod, mc.podName, mc.namespace)
	if err != nil {
		return fmt.Errorf("failed to delete MongoDB client pod: %w", err)
	}

	mc.isReady = false
	logger.Info("MongoDB client pod deleted")
	return nil
}

// waitForPodReady waits for the client pod to be ready
func (mc *MongoClient) waitForPodReady() error {
	timeout := 2 * time.Minute
	interval := 2 * time.Second
	deadline := time.Now().Add(timeout)

	logger.Info(fmt.Sprintf("Starting to wait for pod %s in namespace %s to be ready", mc.podName, mc.namespace))

	for time.Now().Before(deadline) {
		logger.Debug(fmt.Sprintf("Attempting to fetch pod %s from namespace %s", mc.podName, mc.namespace))

		// Use your existing FetchResource method
		podInterface, err := mc.cluster.FetchResource(kube.Pod, mc.podName, mc.namespace)
		if err != nil {
			logger.Info(fmt.Sprintf("Error fetching pod %s: %v", mc.podName, err))
			time.Sleep(interval)
			continue
		}

		logger.Info(fmt.Sprintf("Successfully fetched pod, attempting to cast to Pod object"))

		pod, ok := podInterface.(*v1.Pod)
		if !ok {
			logger.Info(fmt.Sprintf("Failed to cast pod object, got type: %T", podInterface))
			time.Sleep(interval)
			continue
		}

		logger.Info(fmt.Sprintf("Successfully cast pod object. Pod name: %s, namespace: %s, phase: %s",
			pod.Name, pod.Namespace, pod.Status.Phase))

		// Use the same logic as your existing isPodReady function
		if pod.Status.Phase == v1.PodRunning {
			logger.Info(fmt.Sprintf("Pod %s is running, checking conditions", mc.podName))
			for _, condition := range pod.Status.Conditions {
				logger.Debug(fmt.Sprintf("Pod condition: Type=%s, Status=%s", condition.Type, condition.Status))
				if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
					logger.Info(fmt.Sprintf("Pod %s is ready!", mc.podName))
					return nil
				}
			}
		}

		logger.Info(fmt.Sprintf("Pod %s not ready yet, current phase: %s", mc.podName, pod.Status.Phase))
		time.Sleep(interval)
	}

	return fmt.Errorf("timeout waiting for pod %s to be ready after %v", mc.podName, timeout)
}

// ExecMongoCommand executes a MongoDB command using the client pod
func (mc *MongoClient) ExecMongoCommand(mongoHost, command string) error {
	if !mc.isReady {
		return fmt.Errorf("MongoDB client pod is not ready")
	}

	var out, errOut bytes.Buffer

	// Build the mongosh command with connection to the specified host
	cmd := []string{
		mongoshCommand,
		fmt.Sprintf("mongodb://my-user:password@%s:27017/admin?authSource=admin", mongoHost),
		"--eval", command,
		"--quiet",
	}

	err := mc.cluster.ExecIntoPod(mc.namespace, mc.podName, "mongosh-client", cmd, &out, &errOut)
	if err != nil {
		return fmt.Errorf("failed to execute MongoDB command: %w, stderr: %s", err, errOut.String())
	}

	logger.Debug(fmt.Sprintf("MongoDB command output: %s", out.String()))
	if errOut.Len() > 0 {
		logger.Debug(fmt.Sprintf("MongoDB command stderr: %s", errOut.String()))
	}

	return nil
}

// ExecMongoScript executes a MongoDB script using the client pod
func (mc *MongoClient) ExecMongoScript(mongoHost, script string) error {
	if !mc.isReady {
		return fmt.Errorf("MongoDB client pod is not ready")
	}

	var out, errOut bytes.Buffer

	logger.Info(fmt.Sprintf("Executing MongoDB script against host: %s", mongoHost))
	logger.Debug(fmt.Sprintf("Script content: %s", script))

	// For --eval, we need to escape quotes and make it a single line
	// Remove newlines and extra spaces
	escapedScript := strings.ReplaceAll(script, "\n", " ")
	escapedScript = strings.ReplaceAll(escapedScript, "\t", " ")
	// Remove multiple spaces
	escapedScript = regexp.MustCompile(`\s+`).ReplaceAllString(escapedScript, " ")
	escapedScript = strings.TrimSpace(escapedScript)

	// Escape double quotes for bash
	escapedScript = strings.ReplaceAll(escapedScript, `"`, `\"`)

	// Build the mongosh command with --eval - note the quotes around the script
	cmdString := fmt.Sprintf(`%s "mongodb://my-user:password@%s:27017/admin?authSource=admin" --eval "%s"`,
		mongoshCommand, mongoHost, escapedScript)

	logger.Debug(fmt.Sprintf("Command string: %s", cmdString))

	cmd := []string{"bash", "-c", cmdString}

	err := mc.cluster.ExecIntoPod(mc.namespace, mc.podName, "mongosh-client", cmd, &out, &errOut)

	// Log all output for debugging
	if out.Len() > 0 {
		logger.Info(fmt.Sprintf("MongoDB script stdout: %s", out.String()))
	}
	if errOut.Len() > 0 {
		logger.Info(fmt.Sprintf("MongoDB script stderr: %s", errOut.String()))
	}

	if err != nil {
		return fmt.Errorf("failed to execute MongoDB script: %w", err)
	}

	return nil
}

// GetMongoHostsFromClient retrieves MongoDB replica set member hosts using the client pod
func (mc *MongoClient) GetMongoHostsFromClient(mongoHost string) ([]string, error) {
	if !mc.isReady {
		return nil, fmt.Errorf("MongoDB client pod is not ready")
	}

	var out, errOut bytes.Buffer

	cmd := []string{
		mongoshCommand,
		fmt.Sprintf("mongodb://my-user:password@%s:27017/admin?authSource=admin", mongoHost),
		"--eval", "JSON.stringify(rs.conf())",
		"--quiet",
	}

	err := mc.cluster.ExecIntoPod(mc.namespace, mc.podName, "mongosh-client", cmd, &out, &errOut)
	if err != nil {
		return nil, fmt.Errorf("failed to exec into client pod: %w", err)
	}
	if errOut.Len() > 0 {
		return nil, fmt.Errorf("mongosh error: %s", errOut.String())
	}

	output := out.String()
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

// GetPrimaryMongoHostFromClient finds the primary MongoDB host using the client pod
func (mc *MongoClient) GetPrimaryMongoHostFromClient(mongoHost string) (string, error) {
	if !mc.isReady {
		return "", fmt.Errorf("MongoDB client pod is not ready")
	}

	var out, errOut bytes.Buffer

	cmd := []string{
		mongoshCommand,
		fmt.Sprintf("mongodb://my-user:password@%s:27017/admin?authSource=admin", mongoHost),
		"--eval", "JSON.stringify(rs.status())",
		"--quiet",
	}

	err := mc.cluster.ExecIntoPod(mc.namespace, mc.podName, "mongosh-client", cmd, &out, &errOut)
	if err != nil {
		return "", fmt.Errorf("failed to exec into client pod: %w", err)
	}
	if errOut.Len() > 0 {
		return "", fmt.Errorf("mongosh error: %s", errOut.String())
	}

	output := out.String()
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
			parts := strings.Split(member.Name, ".")
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("no PRIMARY member found in replica set")
}
