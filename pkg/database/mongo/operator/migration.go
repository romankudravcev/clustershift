package operator

import (
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/helm"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"clustershift/internal/migration"
	"clustershift/pkg/database/mongo/statefulset"
	"context"
	"encoding/json"
	"fmt"
	mongov1 "github.com/mongodb/mongodb-kubernetes-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"strings"
)

const mongoImage = "mongodb/mongodb-kubernetes-operator"

// OperatorInfo holds information about the MongoDB operator deployment
type OperatorInfo struct {
	Version   string
	Namespace string
	Name      string
	IsPresent bool
}

func Migrate(c kube.Clusters, resources migration.Resources) {
	operatorInfo, err := fetchOperatorInfo(c.Origin)
	if err != nil {
		exit.OnErrorWithMessage(err, "Failed to fetch MongoDB operator information")
	}
	if !operatorInfo.IsPresent {
		logger.Debug("No MongoDB Community Operator found in origin cluster, skipping operator migration")
		return
	}

	logger.Debug(fmt.Sprintf("Found MongoDB Community Operator version %s in namespace %s", operatorInfo.Version, operatorInfo.Namespace))
	deployOperatorToTarget(c.Target, operatorInfo)

	mongoDBs, err := scanExistingDatabases(c.Origin)
	exit.OnErrorWithMessage(err, "Failed to scan existing MongoDB databases")
	logger.Debug(fmt.Sprintf("Found %d MongoDB databases in origin cluster", len(mongoDBs)))

	for _, mongoDB := range mongoDBs {
		err := deployMongoDBCluster(c.Target, mongoDB)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to deploy MongoDB cluster %s in target cluster", mongoDB.Name))

		waitForMongoDbToBeReady(c.Target, mongoDB.Name, mongoDB.Namespace)

		service, err := getServiceForStatefulSet(mongoDB, c.Origin)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to get service for MongoDB cluster %s in origin cluster", mongoDB.Name))
		resources.ExportService(c.Origin, service.Namespace, service.Name)
		resources.ExportService(c.Target, service.Namespace, service.Name)

		err = statefulset.CreateSyncUser(c.Target, mongoDB.Name+"-0", mongoDB.Namespace)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to create sync user for MongoDB cluster %s in target cluster", mongoDB.Name))

		deployMongoSyncer(c.Target)

		// TODO: Wait for Mongosyncer to be done
		// check if job with name "mongosyncer" is done
		jobName := "mongosyncer"
		job, err := c.Target.Clientset.BatchV1().Jobs("default").Get(context.TODO(), jobName, metav1.GetOptions{})
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to get job %s in target cluster", jobName))
		if job.Status.Succeeded == 0 {

		}
	}

}

func deployMongoSyncer(c kube.Cluster) {
	config := map[string]string{
		//"MONGOSYNC_SOURCE": fmt.Sprintf("%s.%s.svc.cluster.local:27017", constants.MongoSyncerSourceService, constants.MongoSyncerSourceNamespace),
		//"MONGOSYNC_TARGET": fmt.Sprintf("%s.%s.svc.cluster.local:27017", constants.MongoSyncerTargetService, constants.MongoSyncerTargetNamespace),
	}

	configMap := &corev1.ConfigMap{

		ObjectMeta: metav1.ObjectMeta{
			Name:      "mongosyncer-config",
			Namespace: "default",
		},
		Data: config,
	}

	_, err := c.Clientset.CoreV1().ConfigMaps("default").Create(context.TODO(), configMap, metav1.CreateOptions{})
	exit.OnErrorWithMessage(err, "Failed to create MongoSyncer ConfigMap")

	err = c.CreateResourcesFromURL(constants.MongoSyncerURL, "default")
	exit.OnErrorWithMessage(err, "Failed to deploy MongoSyncer")
}

func getServiceForStatefulSet(mongo mongov1.MongoDBCommunity, c kube.Cluster) (corev1.Service, error) {
	ns := mongo.Namespace

	services, err := c.Clientset.CoreV1().Services(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return corev1.Service{}, fmt.Errorf("failed to list services: %w", err)
	}

	for _, svc := range services.Items {
		if reflect.DeepEqual(svc.OwnerReferences[0].Name, mongo.Name) {
			return svc, nil
		}
	}

	return corev1.Service{}, fmt.Errorf("no matching service found for statefulset %s", mongo.Name)
}

func waitForMongoDbToBeReady(c kube.Cluster, name string, namespace string) {
	logger.Debug(fmt.Sprintf("Waiting for MongoDB cluster %s in namespace %s to be ready", name, namespace))
	for {
		resource, err := c.FetchCustomResource(
			"mongodbcommunity.mongodb.com",
			"v1",
			"MongoDBCommunity",
			namespace,
			name,
		)
		exit.OnErrorWithMessage(err, "Failed to fetch MongoDB Community resources")

		mongoDB := &mongov1.MongoDBCommunity{}
		jsonData, err := json.Marshal(resource)
		if err != nil {
			exit.OnErrorWithMessage(err, "Failed to marshal MongoDB Community resource")
		}

		if err := json.Unmarshal(jsonData, mongoDB); err != nil {
			exit.OnErrorWithMessage(err, "Failed to unmarshal MongoDB Community resource")
		}

		if mongoDB.Status.Phase == mongov1.Running {
			logger.Debug(fmt.Sprintf("MongoDB cluster %s in namespace %s is ready", name, namespace))
			return
		}

		logger.Debug(fmt.Sprintf("MongoDB cluster %s in namespace %s is not ready yet, current phase: %s", name, namespace, mongoDB.Status.Phase))
	}
}

// fetchOperatorInfo checks if MongoDB Community Operator is deployed and fetches its version
func fetchOperatorInfo(c kube.Cluster) (*OperatorInfo, error) {
	logger.Info("Checking for existing MongoDB Community Operator deployment")

	deployments, err := c.Clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{})
	exit.OnErrorWithMessage(err, "Failed to list statefulsets")

	for _, deployment := range deployments.Items {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if strings.Contains(container.Image, mongoImage) {
				version := extractOperatorVersion(container)

				return &OperatorInfo{
					Version:   version,
					Namespace: deployment.Namespace,
					Name:      deployment.Name,
					IsPresent: true,
				}, nil
			}
		}
	}

	return &OperatorInfo{
		Version:   "",
		Namespace: "",
		Name:      "",
		IsPresent: false,
	}, nil
}

// deployOperatorToTarget deploys the MongoDB Community Operator to the target cluster with the same version
func deployOperatorToTarget(c kube.Cluster, operatorInfo *OperatorInfo) {
	logger.Info(fmt.Sprintf("Deploying MongoDB Community Operator version %s to target cluster", operatorInfo.Version))

	helmOptions := helm.HelmClientOptions{
		KubeConfigPath: c.ClusterOptions.KubeconfigPath,
		Context:        c.ClusterOptions.Context,
		Namespace:      operatorInfo.Namespace,
		Debug:          constants.Debug,
	}

	helmClient := helm.GetHelmClient(helmOptions)

	chartOptions := helm.ChartOptions{
		RepoName:    constants.MongoDBOperatorRepoName,
		RepoURL:     constants.MongoDBOperatorRepoURL,
		ReleaseName: constants.MongoDBOperatorChartName,
		Namespace:   operatorInfo.Namespace,
		ChartName:   constants.MongoDBOperatorRepoName + "/" + constants.MongoDBOperatorChartName,
		Wait:        true,
		Version:     operatorInfo.Version,
	}

	helm.HelmAddandInstallChart(helmClient, chartOptions)
}

// extractOperatorVersion extracts version information from the operator deployment
func extractOperatorVersion(container corev1.Container) string {
	image := container.Image

	if strings.Contains(image, ":") {
		parts := strings.Split(image, ":")
		if len(parts) >= 2 {
			tag := parts[len(parts)-1]
			// Remove any additional suffixes like "-ubi"
			if strings.Contains(tag, "-") {
				tag = strings.Split(tag, "-")[0]
			}
			return tag
		}
	}
	return "latest"
}

func scanExistingDatabases(c kube.Cluster) ([]mongov1.MongoDBCommunity, error) {
	resources, err := c.FetchCustomResources(
		"mongodbcommunity.mongodb.com",
		"v1",
		"MongoDBCommunity",
	)
	exit.OnErrorWithMessage(err, "Failed to fetch MongoDB Community resources")

	var mongoDBs []mongov1.MongoDBCommunity
	for _, resource := range resources {
		jsonData, err := json.Marshal(resource)
		if err != nil {
			return nil, err
		}

		mongoDB := &mongov1.MongoDBCommunity{}
		if err := json.Unmarshal(jsonData, mongoDB); err != nil {
			return nil, err
		}

		mongoDBs = append(mongoDBs, *mongoDB)

	}
	return mongoDBs, nil
}

func deployMongoDBCluster(c kube.Cluster, mongoDB mongov1.MongoDBCommunity) error {
	logger.Debug(fmt.Sprintf("Deploying MongoDB cluster %s in namespace %s", mongoDB.Name, mongoDB.Namespace))

	jsonData, err := json.Marshal(mongoDB)
	if err != nil {
		return err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	err = c.CreateCustomResource(mongoDB.Namespace, data)
	if err != nil {
		return err
	}

	return nil
}
