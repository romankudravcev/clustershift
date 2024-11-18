package kube

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func WaitForPodsReady(c Cluster, labelSelector string, namespace string, timeout time.Duration) error {
	start := time.Now()
	for {
		// Check if timeout exceeded
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for pods to be ready after %v", timeout)
		}

		// Check origin cluster pods
		pods, _ := c.Clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})

		// Check if pods exist and are running
		podReady := isPodRunning(pods)

		if podReady {
			return nil
		}

		// Wait before next check
		time.Sleep(5 * time.Second)
	}
}

func isPodRunning(pods *corev1.PodList) bool {
	if len(pods.Items) == 0 {
		return false
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			return false
		}
	}
	return true
}
