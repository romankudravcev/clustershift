package connectivity

import (
	"clustershift/internal/cli"
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func DiagnoseConnection(kubeconfigOrigin string, kubeconfigTarget string) {
	logger := cli.NewLogger("Running connectivity probe", nil)

	clusters, err := kube.InitClients(kubeconfigOrigin, kubeconfigTarget)
	exit.OnErrorWithMessage(logger.Fail("Error initializing kubernetes clients", err))

	RunClusterConnectivityProbe(clusters, logger)
}

func RunClusterConnectivityProbe(clusters kube.Clusters, logger *cli.Logger) {
	l := logger.Log("Fetching cluster IPs")

	// Get IPs arrays
	originClusterIPs, err := getClusterIP(clusters.Origin.Clientset)
	exit.OnErrorWithMessage(l.Fail("Error getting origin cluster IPs", err))

	targetClusterIPs, err := getClusterIP(clusters.Target.Clientset)
	exit.OnErrorWithMessage(l.Fail("Error getting target cluster IPs", err))

	l.Success("Fetched cluster IPs")

	// Try each combination of IPs
	for _, originIP := range originClusterIPs {
		for _, targetIP := range targetClusterIPs {
			cleanupResources(&clusters, constants.ConnectivityProbeNamespace, l)
			l = logger.Log(fmt.Sprintf("Testing connectivity with Origin IP: %s, Target IP: %s", originIP, targetIP))

			l1 := l.Log("Deploying probe resources")
			// Create namespace if it doesn't exist in both clusters
			clusters.Origin.CreateNewNamespace(constants.ConnectivityProbeNamespace)
			clusters.Target.CreateNewNamespace(constants.ConnectivityProbeNamespace)

			// Create configmaps with the current IP combination
			originConfigMap := createConfigMap(constants.ConnectivityProbeConfigmapName,
				constants.ConnectivityProbeNamespace,
				targetIP,
				"6443")

			targetConfigMap := createConfigMap(constants.ConnectivityProbeConfigmapName,
				constants.ConnectivityProbeNamespace,
				originIP,
				"6443")

			clusters.Origin.CreateResource(kube.ConfigMap,
				constants.ConnectivityProbeConfigmapName,
				constants.ConnectivityProbeNamespace,
				originConfigMap)

			clusters.Target.CreateResource(kube.ConfigMap,
				constants.ConnectivityProbeConfigmapName,
				constants.ConnectivityProbeNamespace,
				targetConfigMap)

			// Create deployments
			err = clusters.Origin.CreateResourcesFromURL(constants.ConnectivityProbeDeploymentURL)
			if err != nil {
				l1.Warning("Failed to create resources", err)
				cleanupResources(&clusters, constants.ConnectivityProbeNamespace, l)
				continue
			}

			err = clusters.Target.CreateResourcesFromURL(constants.ConnectivityProbeDeploymentURL)
			if err != nil {
				l1.Warning("Failed to create resources", err)
				cleanupResources(&clusters, constants.ConnectivityProbeNamespace, l)
				continue
			}

			l1.Success("Deployed probe resources")

			// Check if the pods are running
			l1 = l.Log("Waiting for pods to be ready")
			err = waitForPodsReady(
				clusters.Origin.Clientset,
				clusters.Target.Clientset,
				constants.ConnectivityProbeDeploymentName,
				constants.ConnectivityProbeNamespace,
				90*time.Second,
			)
			if err != nil {
				l1.Warning("Failed waiting for pods", err)
				cleanupResources(&clusters, constants.ConnectivityProbeNamespace, l)
				continue
			}
			l1.Success("Pods are ready")

			// Check connectivity
			l1 = l.Log("Checking connectivity between clusters")

			// Give pods a few seconds to start probing
			time.Sleep(10 * time.Second)

			// Check Origin -> Target connectivity
			originSuccess, err := checkConnectivityProbeLogs(&clusters.Origin, constants.ConnectivityProbeDeploymentName, constants.ConnectivityProbeNamespace)
			if err != nil {
				l1.Warning("Failed to check origin cluster logs", err)
				cleanupResources(&clusters, constants.ConnectivityProbeNamespace, l)
				continue
			}

			// Check Target -> Origin connectivity
			targetSuccess, err := checkConnectivityProbeLogs(&clusters.Target, constants.ConnectivityProbeDeploymentName, constants.ConnectivityProbeNamespace)
			if err != nil {
				l1.Warning("Failed to check target cluster logs", err)
				cleanupResources(&clusters, constants.ConnectivityProbeNamespace, l)
				continue
			}

			if originSuccess && targetSuccess {
				l1.Success(fmt.Sprintf("Connectivity check successful with Origin IP: %s, Target IP: %s - both clusters can reach each other", originIP, targetIP))
			} else {
				if !originSuccess {
					err = fmt.Errorf("Origin cluster (%s) cannot reach target cluster (%s)", originIP, targetIP)
					l1.Warning("Connectivity check failed", err)
				}
				if !targetSuccess {
					err = fmt.Errorf("Target cluster (%s) cannot reach origin cluster (%s)", targetIP, originIP)
					l1.Warning("Connectivity check failed", err)
				}
				continue
			}

			cleanupResources(&clusters, constants.ConnectivityProbeNamespace, logger)
			l.Success("Connectivity test successful")
			logger.Success("Connectivity probe complete")
		}
	}

	exit.OnErrorWithMessage(l.Fail("All IP combinations failed connectivity check", fmt.Errorf("")))
}

func getClusterIP(client *kubernetes.Clientset) ([]string, error) {
	ips := []string{}
	// Get the Kubernetes API server endpoint
	nodes, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return ips, err
	}

	if len(nodes.Items) == 0 {
		return ips, fmt.Errorf("no nodes found in the cluster")
	}

	// Get the first node's external IP
	for _, addr := range nodes.Items[0].Status.Addresses {
		if addr.Type == corev1.NodeExternalIP {
			ips = append(ips, addr.Address)
		}
	}

	// Fallback to internal IP if external IP is not available
	for _, addr := range nodes.Items[0].Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			ips = append(ips, addr.Address)
		}
	}

	if len(ips) == 0 {
		return ips, fmt.Errorf("no suitable IP address found for the cluster")
	}

	return ips, nil
}

func createConfigMap(name string, namespace string, targetIP string, port string) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"target": targetIP,
			"port":   port,
		},
	}
}

func waitForPodsReady(originClient, targetClient *kubernetes.Clientset, name string, namespace string, timeout time.Duration) error {
	start := time.Now()
	for {
		// Check if timeout exceeded
		if time.Since(start) > timeout {
			return fmt.Errorf("timeout waiting for pods to be ready after %v", timeout)
		}

		// Check origin cluster pods
		originPods, _ := originClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=" + name,
		})

		// Check target cluster pods
		targetPods, _ := targetClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: "app=" + name,
		})

		// Check if pods exist and are running
		originReady := isPodRunning(originPods)
		targetReady := isPodRunning(targetPods)

		if originReady && targetReady {
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

func checkConnectivityProbeLogs(cluster *kube.Cluster, name string, namespace string) (bool, error) {
	// Get the pod
	pods, err := cluster.Clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: "app=" + name,
	})
	if err != nil {
		return false, fmt.Errorf("failed to list pods: %v", err)
	}

	if len(pods.Items) == 0 {
		return false, fmt.Errorf("no pods found")
	}

	// Get logs from the pod
	logs, err := cluster.Clientset.CoreV1().Pods(namespace).GetLogs(pods.Items[0].Name, &corev1.PodLogOptions{}).Do(context.TODO()).Raw()
	if err != nil {
		return false, fmt.Errorf("failed to get pod logs: %v", err)
	}

	// Check if logs contain successful connection message
	// You might want to adjust this based on what your probe outputs
	return strings.Contains(string(logs), "Successfully connected to"), nil
}

func cleanupResources(clusters *kube.Clusters, namespace string, logger *cli.Logger) {
	l := logger.Log("Cleaning up probe resources")

	// Delete namespaces in both clusters
	deletePolicy := metav1.DeletePropagationForeground
	deleteOptions := metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	}

	err := clusters.Origin.Clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, deleteOptions)
	exit.OnErrorWithMessage(l.Fail("Failed to cleanup origin cluster namespace", err))
	err = clusters.Target.Clientset.CoreV1().Namespaces().Delete(context.TODO(), namespace, deleteOptions)
	exit.OnErrorWithMessage(l.Fail("Failed to cleanup target cluster namespace", err))
	l.Success("Cleaned up probe resources")
}