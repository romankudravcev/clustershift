package mongo

import (
	"bytes"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mongoClientPodName = "mongosh-client"
	mongoClientImage   = "mongo:latest"
	mongoshCommand     = "mongosh"
	primaryState       = "PRIMARY"
	secondaryState     = "SECONDARY"
)

// Client manages a MongoDB client pod for executing commands
type Client struct {
	Cluster   kube.Cluster
	Namespace string
	PodName   string
	IsReady   bool
}

// NewMongoClient creates a new MongoDB client instance
func NewMongoClient(cluster kube.Cluster, namespace string) *Client {
	mongoClient := &Client{
		Cluster:   cluster,
		Namespace: namespace,
		PodName:   mongoClientPodName,
		IsReady:   false,
	}

	err := mongoClient.CreateClientPod()
	exit.OnErrorWithMessage(err, "Failed to create MongoDB client pod")

	return mongoClient

}

// CreateClientPod creates a MongoDB client pod for executing commands
func (mc *Client) CreateClientPod() error {
	logger.Debug("Creating MongoDB client pod...")

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mc.PodName,
			Namespace: mc.Namespace,
			Labels: map[string]string{
				"app":  "mongosh-client",
				"role": "database-client",
			},
		},
		Spec: v1.PodSpec{
			RestartPolicy: v1.RestartPolicyNever,
			Containers: []v1.Container{
				{
					Name:    "clustershift-mongosh-client",
					Image:   mongoClientImage,
					Command: []string{"sleep", "3600"},
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

	err := mc.Cluster.CreateResource(kube.Pod, mc.Namespace, pod)
	if err != nil {
		return fmt.Errorf("failed to create MongoDB client pod: %w", err)
	}

	logger.Debug("Waiting for MongoDB client pod to be ready...")
	err = kube.WaitForPodReadyByName(mc.Cluster, mc.PodName, mc.Namespace, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("MongoDB client pod failed to become ready: %w", err)
	}

	mc.IsReady = true
	return nil
}

// DeleteClientPod deletes the MongoDB client pod
func (mc *Client) DeleteClientPod() error {
	if !mc.IsReady {
		return nil
	}

	logger.Debug("Deleting MongoDB client pod...")

	err := mc.Cluster.DeleteResource(kube.Pod, mc.PodName, mc.Namespace)
	if err != nil {
		return fmt.Errorf("failed to delete MongoDB client pod: %w", err)
	}

	mc.IsReady = false
	return nil
}

// execMongoCommand executes a MongoDB command using the client pod
func (mc *Client) ExecMongoCommand(command []string) (string, error) {
	if !mc.IsReady {
		return "", fmt.Errorf("MongoDB client pod is not ready")
	}

	var out, errOut bytes.Buffer

	err := mc.Cluster.ExecIntoPod(mc.Namespace, mc.PodName, "", command, &out, &errOut)
	if err != nil {
		return "", fmt.Errorf("failed to execute MongoDB command: %w, stderr: %s", err, errOut.String())
	}

	logger.Debug(fmt.Sprintf("MongoDB command output: %s", out.String()))
	if errOut.Len() > 0 {
		return "", fmt.Errorf("%s", errOut.String())
	}

	return out.String(), nil
}
