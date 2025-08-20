package linkerd

import (
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"context"
	"fmt"
	v1 "k8s.io/api/apps/v1"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"time"
)

func ExportService(cluster kube.Cluster, name, namespace string) {
	logger.Info(fmt.Sprintf("Exporting service %s in namespace %s", name, namespace))

	// Add linkerd.io/inject=enabled annotation to the namespace
	logger.Info(fmt.Sprintf("Adding linkerd.io/inject=enabled annotation to namespace %s", namespace))

	// Fetch the namespace object first
	namespaceInterface, err := cluster.FetchResource(kube.Namespace, namespace, "")
	exit.OnErrorWithMessage(err, "Failed to fetch namespace")
	namespaceObj := namespaceInterface.(*v1core.Namespace)

	// Check if the namespace already has the linkerd.io/inject annotation
	if namespaceObj.Annotations == nil || namespaceObj.Annotations["linkerd.io/inject"] != "enabled" {
		// Add the linkerd injection annotation
		err = cluster.AddAnnotation(namespaceObj, "linkerd.io/inject", "enabled")
		exit.OnErrorWithMessage(err, "Failed to add linkerd inject annotation to namespace")

		// Reroll all pods in the namespace by restarting deployments and statefulsets
		logger.Info(fmt.Sprintf("Rerolling all pods in namespace %s", namespace))
		err = RerollPodsInNamespace(cluster, namespace)
	}

	exit.OnErrorWithMessage(err, "Failed to reroll pods in namespace")

	mirrorLabel := map[string]string{
		"mirror.linkerd.io/exported": "true",
	}

	err = cluster.AddLabel(kube.Service, name, namespace, mirrorLabel)
	exit.OnErrorWithMessage(err, "Failed to export service")
}

func MirrorService(cluster kube.Cluster, name, namespace string) error {
	logger.Info(fmt.Sprintf("Mirroring service %s in namespace %s", name, namespace))

	mirrorLabel := map[string]string{
		"mirror.linkerd.io/exported": "true",
	}

	err := cluster.AddLabel(kube.Service, name, namespace, mirrorLabel)
	if err != nil {
		return fmt.Errorf("failed to mirror service: %v", err)
	}

	return nil
}

func InjectNamespace(cluster kube.Cluster, namespace string) error {
	logger.Info(fmt.Sprintf("Injecting namespace %s with linkerd", namespace))

	// Fetch the namespace object first
	namespaceInterface, err := cluster.FetchResource(kube.Namespace, namespace, "")
	if err != nil {
		return fmt.Errorf("failed to fetch namespace %s: %v", namespace, err)
	}
	namespaceObj := namespaceInterface.(*v1core.Namespace)

	// Check if the namespace already has the linkerd.io/inject annotation
	if namespaceObj.Annotations == nil || namespaceObj.Annotations["linkerd.io/inject"] != "enabled" {
		// Add the linkerd injection annotation
		err = cluster.AddAnnotation(namespaceObj, "linkerd.io/inject", "enabled")
		if err != nil {
			return fmt.Errorf("failed to add linkerd inject annotation to namespace %s: %v", namespace, err)
		}

		// Reroll all pods in the namespace by restarting deployments and statefulsets
		logger.Info(fmt.Sprintf("Rerolling all pods in namespace %s", namespace))
		err = RerollPodsInNamespace(cluster, namespace)
		if err != nil {
			return fmt.Errorf("failed to reroll pods in namespace %s: %v", namespace, err)
		}
	}

	return nil
}

// rerollPodsInNamespace restarts all deployments and statefulsets in a namespace
// This triggers a rolling update which ensures zero downtime
func RerollPodsInNamespace(cluster kube.Cluster, namespace string) error {
	// Restart all deployments in the namespace
	deploymentsInterface, err := cluster.FetchResources(kube.Deployment)
	if err != nil {
		return fmt.Errorf("failed to fetch deployments: %v", err)
	}

	deployments := deploymentsInterface.(*v1.DeploymentList)
	var deploymentNames []string
	for _, deployment := range deployments.Items {
		if deployment.Namespace == namespace {
			logger.Info(fmt.Sprintf("Restarting deployment %s in namespace %s", deployment.Name, namespace))
			err := restartDeployment(cluster, deployment.Name, namespace)
			if err != nil {
				return fmt.Errorf("failed to restart deployment %s: %v", deployment.Name, err)
			}
			deploymentNames = append(deploymentNames, deployment.Name)
		}
	}

	// Restart all statefulsets in the namespace
	statefulsetsInterface, err := cluster.FetchResources(kube.StatefulSet)
	if err != nil {
		return fmt.Errorf("failed to fetch statefulsets: %v", err)
	}

	statefulsets := statefulsetsInterface.(*v1.StatefulSetList)
	var statefulsetNames []string
	for _, statefulset := range statefulsets.Items {
		if statefulset.Namespace == namespace {
			logger.Info(fmt.Sprintf("Restarting statefulset %s in namespace %s", statefulset.Name, namespace))
			err := restartStatefulSet(cluster, statefulset.Name, namespace)
			if err != nil {
				return fmt.Errorf("failed to restart statefulset %s: %v", statefulset.Name, err)
			}
			statefulsetNames = append(statefulsetNames, statefulset.Name)
		}
	}

	// Restart all CNPG clusters in the namespace
	cnpgClusters, err := cluster.FetchCustomResources("postgresql.cnpg.io", "v1", "clusters")
	if err != nil {
		logger.Info(fmt.Sprintf("No CNPG clusters found or error fetching them: %v", err))
	} else {
		var cnpgClusterNames []string
		for _, cnpgCluster := range cnpgClusters {
			metadata, ok := cnpgCluster["metadata"].(map[string]interface{})
			if !ok {
				continue
			}

			clusterNamespace, ok := metadata["namespace"].(string)
			if !ok || clusterNamespace != namespace {
				continue
			}

			clusterName, ok := metadata["name"].(string)
			if !ok {
				continue
			}

			logger.Info(fmt.Sprintf("Restarting CNPG cluster %s in namespace %s", clusterName, namespace))
			err := restartCNPGCluster(cluster, clusterName, namespace)
			if err != nil {
				return fmt.Errorf("failed to restart CNPG cluster %s: %v", clusterName, err)
			}
			cnpgClusterNames = append(cnpgClusterNames, clusterName)
		}

		// Wait for all CNPG clusters to be ready
		logger.Info(fmt.Sprintf("Waiting for CNPG clusters to be ready in namespace %s", namespace))
		for _, cnpgClusterName := range cnpgClusterNames {
			err := WaitForCNPGClusterReady(cluster.DynamicClientset, cnpgClusterName, namespace, 15*time.Minute)
			if err != nil {
				return fmt.Errorf("failed to wait for CNPG cluster %s to be ready: %v", cnpgClusterName, err)
			}
			logger.Info(fmt.Sprintf("CNPG cluster %s is ready", cnpgClusterName))
		}
	}

	// Wait for all deployments to be ready
	logger.Info(fmt.Sprintf("Waiting for deployments to be ready in namespace %s", namespace))
	for _, deploymentName := range deploymentNames {
		err := waitForDeploymentReady(cluster, deploymentName, namespace, 10*time.Minute)
		if err != nil {
			return fmt.Errorf("failed to wait for deployment %s to be ready: %v", deploymentName, err)
		}
		logger.Info(fmt.Sprintf("Deployment %s is ready", deploymentName))
	}

	// Wait for all statefulsets to be ready
	logger.Info(fmt.Sprintf("Waiting for statefulsets to be ready in namespace %s", namespace))
	for _, statefulsetName := range statefulsetNames {
		err := waitForStatefulSetReady(cluster, statefulsetName, namespace, 10*time.Minute)
		if err != nil {
			return fmt.Errorf("failed to wait for statefulset %s to be ready: %v", statefulsetName, err)
		}
		logger.Info(fmt.Sprintf("StatefulSet %s is ready", statefulsetName))
	}

	return nil
}

// restartDeployment performs a rolling restart of a deployment by updating the restart annotation
func restartDeployment(cluster kube.Cluster, name, namespace string) error {
	deploymentInterface, err := cluster.FetchResource(kube.Deployment, name, namespace)
	if err != nil {
		return fmt.Errorf("failed to fetch deployment: %v", err)
	}

	deployment := deploymentInterface.(*v1.Deployment)

	// Add restart annotation to trigger rolling update
	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	// Update the deployment
	_, err = cluster.Clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment: %v", err)
	}

	return nil
}

// restartStatefulSet performs a rolling restart of a statefulset by updating the restart annotation
func restartStatefulSet(cluster kube.Cluster, name, namespace string) error {
	statefulsetInterface, err := cluster.FetchResource(kube.StatefulSet, name, namespace)
	if err != nil {
		return fmt.Errorf("failed to fetch statefulset: %v", err)
	}

	statefulset := statefulsetInterface.(*v1.StatefulSet)

	// Add restart annotation to trigger rolling update
	if statefulset.Spec.Template.Annotations == nil {
		statefulset.Spec.Template.Annotations = make(map[string]string)
	}
	statefulset.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

	// Update the statefulset
	_, err = cluster.Clientset.AppsV1().StatefulSets(namespace).Update(context.TODO(), statefulset, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update statefulset: %v", err)
	}

	return nil
}

// restartCNPGCluster performs a rolling restart of a CNPG cluster by updating the restart annotation
func restartCNPGCluster(cluster kube.Cluster, name, namespace string) error {
	gvr := schema.GroupVersionResource{
		Group:    "postgresql.cnpg.io",
		Version:  "v1",
		Resource: "clusters",
	}

	// Get the current CNPG cluster
	cnpgCluster, err := cluster.DynamicClientset.Resource(gvr).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to fetch CNPG cluster: %v", err)
	}

	// Add restart annotation to trigger rolling update
	annotations := cnpgCluster.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
	cnpgCluster.SetAnnotations(annotations)

	// Update the CNPG cluster
	_, err = cluster.DynamicClientset.Resource(gvr).Namespace(namespace).Update(context.TODO(), cnpgCluster, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update CNPG cluster: %v", err)
	}

	return nil
}

// WaitForCNPGClusterReady waits for a CNPG cluster to be ready after a rolling update
func WaitForCNPGClusterReady(dynamicClient dynamic.Interface, clusterName string, namespace string, timeout time.Duration) error {
	return kube.WaitForCNPGClusterReady(dynamicClient, clusterName, namespace, timeout)
}

// waitForDeploymentReady waits for a deployment to be ready after a rolling update
func waitForDeploymentReady(cluster kube.Cluster, name, namespace string, timeout time.Duration) error {
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for deployment %s to be ready after %v", name, timeout)
		case <-ticker.C:
			deploymentInterface, err := cluster.FetchResource(kube.Deployment, name, namespace)
			if err != nil {
				return fmt.Errorf("failed to fetch deployment %s: %v", name, err)
			}

			deployment := deploymentInterface.(*v1.Deployment)
			if isDeploymentReady(deployment) {
				return nil
			}
		}
	}
}

// waitForStatefulSetReady waits for a statefulset to be ready after a rolling update
func waitForStatefulSetReady(cluster kube.Cluster, name, namespace string, timeout time.Duration) error {
	timeoutCh := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for statefulset %s to be ready after %v", name, timeout)
		case <-ticker.C:
			statefulsetInterface, err := cluster.FetchResource(kube.StatefulSet, name, namespace)
			if err != nil {
				return fmt.Errorf("failed to fetch statefulset %s: %v", name, err)
			}

			statefulset := statefulsetInterface.(*v1.StatefulSet)
			if isStatefulSetReady(statefulset) {
				return nil
			}
		}
	}
}

// isDeploymentReady checks if a deployment is ready (all replicas are available and updated)
func isDeploymentReady(deployment *v1.Deployment) bool {
	if deployment.Spec.Replicas == nil {
		return true // No replicas specified, consider ready
	}

	desiredReplicas := *deployment.Spec.Replicas
	return deployment.Status.ReadyReplicas == desiredReplicas &&
		deployment.Status.UpdatedReplicas == desiredReplicas &&
		deployment.Status.AvailableReplicas == desiredReplicas &&
		deployment.Status.ObservedGeneration >= deployment.Generation
}

// isStatefulSetReady checks if a statefulset is ready (all replicas are ready and updated)
func isStatefulSetReady(statefulset *v1.StatefulSet) bool {
	if statefulset.Spec.Replicas == nil {
		return true // No replicas specified, consider ready
	}

	desiredReplicas := *statefulset.Spec.Replicas
	return statefulset.Status.ReadyReplicas == desiredReplicas &&
		statefulset.Status.UpdatedReplicas == desiredReplicas &&
		statefulset.Status.CurrentReplicas == desiredReplicas &&
		statefulset.Status.ObservedGeneration >= statefulset.Generation
}
