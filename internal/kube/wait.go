package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	apiv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
)

func WaitForPodsReadyByLabel(c Cluster, labelSelector string, namespace string, timeout time.Duration) error {
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector,
	}

	watcher, err := c.Clientset.CoreV1().Pods(namespace).Watch(context.TODO(), listOptions)
	if err != nil {
		return fmt.Errorf("error creating watch: %v", err)
	}
	defer watcher.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					continue
				}

				if isPodReady(pod) {
					return nil
				}
			case watch.Error:
				return fmt.Errorf("error watching pod: %v", event.Object)
			}

		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for pods to be ready after %v", timeout)
		}
	}
}

func WaitForPodReadyByName(c Cluster, podName string, namespace string, timeout time.Duration) error {
	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", podName),
	}

	watcher, err := c.Clientset.CoreV1().Pods(namespace).Watch(context.TODO(), listOptions)
	if err != nil {
		return fmt.Errorf("error creating watch: %v", err)
	}
	defer watcher.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				pod, ok := event.Object.(*corev1.Pod)
				if !ok {
					continue
				}

				if pod.Name == podName && pod.Namespace == namespace && isPodReady(pod) {
					return nil
				}
			case watch.Error:
				return fmt.Errorf("error watching pod: %v", event.Object)
			}

		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for pod to be ready after %v", timeout)
		}
	}
}

func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			if condition.Status != corev1.ConditionTrue {
				return false
			}
			break
		}
	}

	return true
}

func WaitForCNPGClusterReady(dynamicClient dynamic.Interface, clusterName string, namespace string, timeout time.Duration) error {
	gvr := schema.GroupVersionResource{
		Group:    "postgresql.cnpg.io",
		Version:  "v1",
		Resource: "clusters",
	}

	// First, get the current state
	cluster, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(context.TODO(), clusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting cluster: %v", err)
	}

	// Check if it's already ready
	currentCluster, err := ConvertToCluster(cluster.Object)
	if err != nil {
		return fmt.Errorf("error converting cluster: %v", err)
	}

	if isCNPGClusterReady(currentCluster) {
		return nil
	}

	// If not ready, start watching
	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", clusterName),
	}

	watcher, err := dynamicClient.Resource(gvr).Namespace(namespace).Watch(context.TODO(), listOptions)
	if err != nil {
		return fmt.Errorf("error creating watch: %v", err)
	}
	defer watcher.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				unstructuredObj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					continue
				}

				cluster, err := ConvertToCluster(unstructuredObj.Object)
				if err != nil {
					return fmt.Errorf("error converting cluster: %v", err)
				}

				if isCNPGClusterReady(cluster) {
					return nil
				}
			case watch.Error:
				return fmt.Errorf("error watching cluster: %v", event.Object)
			}

		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for cluster to be ready after %v", timeout)
		}
	}
}

func isCNPGClusterReady(cluster *apiv1.Cluster) bool {
	for _, condition := range cluster.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
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
