package linkerd

import (
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"context"
	"fmt"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ExportService(cluster kube.Cluster, name, namespace string) {
	logger.Info(fmt.Sprintf("Exporting service %s in namespace %s", name, namespace))

	mirrorLabel := map[string]string{
		"mirror.linkerd.io/exported": "true",
	}

	err := cluster.AddLabel(kube.Service, name, namespace, mirrorLabel)
	exit.OnErrorWithMessage(err, "Failed to export service")
}

func ExportDeployment(cluster kube.Cluster, name, namespace string) {
	logger.Info(fmt.Sprintf("Exporting deployment %s in namespace %s", name, namespace))
	deploymentInterface, err := cluster.FetchResource(kube.Deployment, name, namespace)
	exit.OnErrorWithMessage(err, "Failed to fetch deployment")
	deployment := deploymentInterface.(*v1.Deployment)

	if deployment.Spec.Template.Annotations == nil {
		deployment.Spec.Template.Annotations = make(map[string]string)
	}
	deployment.Spec.Template.Annotations["linkerd.io/inject"] = "enabled"

	_, err = cluster.Clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deployment, metav1.UpdateOptions{})
	exit.OnErrorWithMessage(err, "Failed to update deployment")
}
