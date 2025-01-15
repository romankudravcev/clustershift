package kube

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func WaitForPodsReady(c Cluster, labelSelector string, namespace string, timeout time.Duration) error {
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
