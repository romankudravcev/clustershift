package submariner

import (
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	lhconstants "github.com/submariner-io/lighthouse/pkg/constants"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mcsv1a1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

func Export(c kube.Cluster, namespace string, name string, useClustersetIP string) {
	logger.Info("Checking for namespace")
	_, err := c.Clientset.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	exit.OnErrorWithMessage(err, fmt.Sprintf("Unable to find the Service %q in namespace %q", name, namespace))

	logger.Info("Namespace exists")

	logger.Info("Creating service export resource")

	mcsServiceExport := &mcsv1a1.ServiceExport{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceExport",
			APIVersion: mcsv1a1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	// If user specified the use-clusterset-ip flag
	if useClustersetIP != "" {
		result, err := strconv.ParseBool(useClustersetIP)
		exit.OnErrorWithMessage(err, "use-clusterset-ip must be set to true/false: ")

		mcsServiceExport.SetAnnotations(map[string]string{lhconstants.UseClustersetIP: strconv.FormatBool(result)})
	}

	resourceServiceExport, err := convertToUnstructured(mcsServiceExport)
	exit.OnErrorWithMessage(err, "Failed to convert to Unstructured")

	logger.Debug(fmt.Sprintf("%v", resourceServiceExport))

	err = c.CreateCustomResource(namespace, resourceServiceExport)
	if k8serrors.IsAlreadyExists(err) {
		logger.Info("Service already exported")
		return
	}
	exit.OnErrorWithMessage(err, "Failed to export service")

	logger.Info("Service exported successfully")
}

func convertToUnstructured(serviceExport *mcsv1a1.ServiceExport) (map[string]interface{}, error) {
	// Marshal cluster to JSON
	jsonData, err := json.Marshal(serviceExport)
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
