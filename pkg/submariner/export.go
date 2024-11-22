package submariner

import (
	"clustershift/internal/cli"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	lhconstants "github.com/submariner-io/lighthouse/pkg/constants"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mcsv1a1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

func Export(c kube.Cluster, namespace string, name string, useClustersetIP string, logger *cli.Logger) {
	l := logger.Log("Checking for namespace")
	_, err := c.Clientset.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	exit.OnErrorWithMessage(l.Fail(fmt.Sprintf("Unable to find the Service %q in namespace %q", name, namespace), err))

	l.Success("Namespace exists")

	l = logger.Log("Creating service export resource")

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
		exit.OnErrorWithMessage(l.Fail("use-clusterset-ip must be set to true/false: ", err))

		mcsServiceExport.SetAnnotations(map[string]string{lhconstants.UseClustersetIP: strconv.FormatBool(result)})
	}

	resourceServiceExport, err := convertToUnstructured(mcsServiceExport)
	exit.OnErrorWithMessage(l.Fail("Failed to convert to Unstructured", err))

	cli.LogToFile(fmt.Sprintf("%v", resourceServiceExport))

	err = c.CreateCustomResource(namespace, resourceServiceExport, l)
	if k8serrors.IsAlreadyExists(err) {
		l.Success("Service already exported")
		return
	}
	exit.OnErrorWithMessage(l.Fail("Failed to export service", err))

	l.Success("Service exported successfully")
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
