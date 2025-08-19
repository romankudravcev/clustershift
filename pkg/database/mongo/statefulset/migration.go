package statefulset

import (
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"clustershift/internal/migration"
	"clustershift/internal/mongo"
	"clustershift/internal/prompt"
	"clustershift/pkg/database/postgres"
	"clustershift/pkg/skupper"
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"strings"
	"time"
)

// Migrate migrates MongoDB StatefulSets from origin to target cluster
func Migrate(c kube.Clusters, resources migration.Resources) {
	logger.Info("Migrating MongoDBs")

	statefulSets := scanExistingDatabases(c.Origin)
	if len(statefulSets) == 0 {
		logger.Info("No existing MongoDBs found, skipping migration")
		return
	}

	mongoClientOrigin := mongo.NewMongoClient(c.Origin, "default")
	mongoClientTarget := mongo.NewMongoClient(c.Target, "default")

	for _, statefulSet := range statefulSets {
		ctx, err := prepareMigrationContext(statefulSet, c, resources, mongoClientOrigin)
		if err != nil {
			exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to prepare migration context for StatefulSet %s", statefulSet.Name))
		}

		err = migrateStatefulSet(ctx, c, resources, mongoClientOrigin, mongoClientTarget)
		if err != nil {
			exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to migrate StatefulSet %s", statefulSet.Name))
		}
	}
}

// prepareMigrationContext prepares the migration context for a StatefulSet
func prepareMigrationContext(statefulSet appsv1.StatefulSet, c kube.Clusters, resources migration.Resources, mongoClientOrigin *mongo.Client) (*mongo.MigrationContext, error) {
	service, err := getServiceForStatefulSet(statefulSet, c.Origin)
	if err != nil {
		return nil, fmt.Errorf("failed to get service for StatefulSet %s: %w", statefulSet.Name, err)
	}

	originService := service

	if resources.GetNetworkingTool() == prompt.NetworkingToolSkupper {
		originService.Name = service.Name + "-origin"
		//cleanOriginService := kube.CleanResourceForCreation(originService)
		//err = c.Origin.CreateResource(kube.Service, originService.Namespace, cleanOriginService)
		//exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to create origin service for StatefulSet %s", statefulSet.Name))
	}

	targetService := service
	if resources.GetNetworkingTool() == prompt.NetworkingToolSkupper {
		targetService.Name = service.Name + "-target"
		//cleanTargetService := kube.CleanResourceForCreation(targetService)
		//err = c.Target.CreateResource(kube.Service, targetService.Namespace, cleanTargetService)
		//exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to create target service for StatefulSet %s", statefulSet.Name))
	}

	var primaryHost string
	var originHosts []string

	primaryHost, err = mongo.GetPrimaryMongoHost(mongoClientOrigin, service.Name+"."+service.Namespace+".svc.cluster.local")
	if err != nil {
		return nil, fmt.Errorf("failed to get primary MongoDB host for StatefulSet %s: %w", statefulSet.Name, err)
	}

	originHosts, err = mongo.GetMongoHosts(mongoClientOrigin, service.Name+"."+service.Namespace+".svc.cluster.local")
	if err != nil {
		return nil, fmt.Errorf("failed to get MongoDB hosts for StatefulSet %s: %w", statefulSet.Name, err)
	}

	logger.Debug(fmt.Sprintf("MongoDB hosts for StatefulSet %s: %v", statefulSet.Name, originHosts))

	return &mongo.MigrationContext{
		StatefulSet:   statefulSet,
		Service:       service,
		OriginService: originService,
		TargetService: targetService,
		PrimaryHost:   primaryHost,
		OriginHosts:   originHosts,
		UpdatedHosts:  UpdateMongoHosts(originHosts, resources, service, c.Origin),
		TargetHosts:   UpdateMongoHosts(originHosts, resources, service, c.Target),
	}, nil
}

// migrateStatefulSet performs the complete migration of a MongoDB StatefulSet
func migrateStatefulSet(ctx *mongo.MigrationContext, c kube.Clusters, resources migration.Resources, mongoClientOrigin, mongoClientTarget *mongo.Client) error {

	if resources.GetNetworkingTool() == prompt.NetworkingToolSkupper {
		originalMemberCount := ctx.StatefulSet.Spec.Replicas

		targetDBPod := &mongo.Client{
			Cluster:   c.Target,
			Namespace: ctx.StatefulSet.Namespace,
			PodName:   fmt.Sprintf("%s-0", ctx.StatefulSet.Name),
			IsReady:   true,
		}

		service := ctx.Service
		serviceInterface := interface{}(service)
		serviceInterface = kube.CleanResourceForCreation(serviceInterface)
		service = *serviceInterface.(*v1core.Service)

		statefulSet := ctx.StatefulSet
		statefulSet.Spec.Replicas = &[]int32{1}[0] // Set replica count to 1
		statefulSetInterface := interface{}(statefulSet)
		statefulSetInterface = kube.CleanResourceForCreation(statefulSetInterface)
		statefulSet = *statefulSetInterface.(*appsv1.StatefulSet)

		if err := CreateResourceIfNotExists(c.Target, kube.Service, service.Namespace, &service); err != nil {
			return fmt.Errorf("failed to create service %s in target cluster: %w", service.Name, err)
		}

		if err := CreateResourceIfNotExists(c.Target, kube.StatefulSet, statefulSet.Namespace, &statefulSet); err != nil {
			return fmt.Errorf("failed to create StatefulSet %s in target cluster: %w", statefulSet.Name, err)
		}

		//wait till statefulset is ready
		err := waitForStatefulSetReady(c.Target, statefulSet.Name, statefulSet.Namespace)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to wait for StatefulSet %s to be ready in target cluster", statefulSet.Name))
		skupper.CreateSiteConnection(c, statefulSet.Namespace)
		resources.ExportService(c.Target, service.Namespace, service.Name)
		time.Sleep(5 * time.Second)

		targetHost := fmt.Sprintf("%s-0.%s.%s.svc.cluster.local:27017", statefulSet.Name, service.Name, service.Namespace)
		err = mongo.InitReplicaSet(targetDBPod, targetHost)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to initialize MongoDB replica set for cluster %s in target cluster", statefulSet.Name))

		var db postgres.DatabaseInstance
		getCredentialsFromStatefulSet(c.Target, statefulSet, &db)

		err = mongo.CreateRootUser(targetDBPod, targetHost, db.Username, db.Password)
		// Get primary hosts for both clusters
		logger.Debug("Getting primary MongoDB hosts for origin")
		originPrimary, err := mongo.GetPrimaryMongoHost(mongoClientOrigin, service.Name+"."+service.Namespace+".svc.cluster.local")
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to get primary MongoDB host for cluster %s in origin cluster", statefulSet.Name))
		originPrimaryHost := originPrimary
		logger.Debug("Getting primary MongoDB hosts for target")
		targetPrimary, err := mongo.GetPrimaryMongoHost(mongoClientTarget, service.Name+"."+service.Namespace+".svc.cluster.local")
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to get primary MongoDB host for cluster %s in target cluster", statefulSet.Name))
		targetPrimaryHost := targetPrimary

		logger.Info(fmt.Sprintf("Primary MongoDB host in origin cluster: %s", originPrimaryHost))
		logger.Info(fmt.Sprintf("Primary MongoDB host in target cluster: %s", targetPrimaryHost))
		err = mongo.CreateSyncUser(mongoClientOrigin, originPrimaryHost)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to create sync user for MongoDB cluster %s in origin cluster", statefulSet.Name))
		err = mongo.CreateSyncUser(mongoClientTarget, targetPrimaryHost)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to create sync user for MongoDB cluster %s in target cluster", statefulSet.Name))

		originURI := getMongoURI(c.Origin, service, resources, mongoClientOrigin, originPrimaryHost)
		targetURI := getMongoURI(c.Target, service, resources, mongoClientTarget, targetPrimaryHost) + "&directConnection=true"

		if resources.GetNetworkingTool() == prompt.NetworkingToolSkupper || resources.GetNetworkingTool() == prompt.NetworkingToolLinkerd {
			targetURI = fmt.Sprintf("mongodb://clusteradmin:password1@%s-target.%s.svc.cluster.local:27017/?authSource=admin&directConnection=true", service.Name, service.Namespace)
		}

		deployMongoSyncer(c.Origin, originURI, targetURI)

		waitForJobCompletion(c.Origin, "default", "mongosyncer-job")

		err = restoreMongoDBMemberCount(c.Target, statefulSet.Name, statefulSet.Namespace, int(*originalMemberCount))
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to restore MongoDB member count for StatefulSet %s in target cluster", statefulSet.Name))

		err = waitForStatefulSetReady(c.Target, statefulSet.Name, statefulSet.Namespace)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to wait for StatefulSet %s to be ready in target cluster", statefulSet.Name))

		for i := 1; i < int(*originalMemberCount); i++ {
			logger.Info(fmt.Sprintf("Adding new MongoDB member %s-%d to replica set in target cluster", statefulSet.Name, i))
			err := mongo.AddMongoMember(mongoClientTarget, targetPrimaryHost, fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:27017", statefulSet.Name, i, service.Name, service.Namespace))
			exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to add new MongoDB member %s-%d to replica set in target cluster", statefulSet.Name, i))
		}

		err = mongo.CreateTestUser(mongoClientTarget, targetPrimaryHost)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to create test user for MongoDB cluster %s in target cluster", statefulSet.Name))

		err = mongoClientOrigin.DeleteClientPod()
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to delete MongoDB client pod in origin cluster for cluster %s", statefulSet.Name))
		err = mongoClientTarget.DeleteClientPod()
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to delete MongoDB client pod in target cluster for cluster %s", statefulSet.Name))

	} else {
		if err := setupTargetResources(ctx, c); err != nil {
			return fmt.Errorf("failed to setup target resources: %w", err)
		}

		if err := configureNetworking(ctx, c, resources); err != nil {
			return fmt.Errorf("failed to configure networking: %w", err)
		}
		if err := updateOriginHosts(ctx, mongoClientOrigin); err != nil {
			return fmt.Errorf("failed to update origin hosts: %w", err)
		}

		if err := addTargetMembersToReplicaSet(ctx, mongoClientOrigin); err != nil {
			return fmt.Errorf("failed to add target members to replica set: %w", err)
		}

		if err := transferPrimary(ctx, mongoClientOrigin); err != nil {
			return fmt.Errorf("failed to transfer primary: %w", err)
		}

		if err := mongo.WaitForTargetPrimaryElection(mongoClientOrigin, ctx); err != nil {
			return fmt.Errorf("failed to wait for new primary election: %w", err)
		}

		if err := removeOriginMembers(ctx, mongoClientOrigin, mongoClientTarget); err != nil {
			return fmt.Errorf("failed to remove origin members: %w", err)
		}
	}
	logger.Info(fmt.Sprintf("Successfully migrated MongoDB StatefulSet %s", ctx.StatefulSet.Name))
	return nil
}

func getMongoURI(c kube.Cluster, service v1core.Service, resources migration.Resources, mongoClient *mongo.Client, host string) string {
	hosts, err := mongo.GetMongoHostsAuthenticated(mongoClient, host)
	exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to get MongoDB hosts for service %s in cluster %s", service.Name, c.Name))
	updatedHosts := hosts

	if resources.GetNetworkingTool() == prompt.NetworkingToolSubmariner {
		updatedHosts = UpdateMongoHosts(hosts, resources, service, c)
	}
	uri := fmt.Sprintf(
		"mongodb://clusteradmin:password1@%s/?authSource=admin",
		strings.Join(updatedHosts, ","),
	)
	logger.Info(uri)
	return uri
}

func waitForStatefulSetReady(cluster kube.Cluster, name string, namespace string) error {
	timeout := 10 * time.Minute

	// First, check if the StatefulSet is already ready
	statefulSetInterface, err := cluster.FetchResource(kube.StatefulSet, name, namespace)
	if err != nil {
		return fmt.Errorf("failed to fetch StatefulSet %s: %w", name, err)
	}

	statefulSet := statefulSetInterface.(*appsv1.StatefulSet)
	if isStatefulSetReady(statefulSet) {
		return nil
	}

	logger.Info(fmt.Sprintf("Waiting for StatefulSet %s to be ready...", name))

	// Start watching for changes
	listOptions := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", name),
	}

	watcher, err := cluster.Clientset.AppsV1().StatefulSets(namespace).Watch(context.TODO(), listOptions)
	if err != nil {
		return fmt.Errorf("error creating watch for StatefulSet %s: %w", name, err)
	}
	defer watcher.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed while waiting for StatefulSet %s", name)
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				statefulSet, ok := event.Object.(*appsv1.StatefulSet)
				if !ok {
					continue
				}

				if isStatefulSetReady(statefulSet) {
					logger.Info(fmt.Sprintf("StatefulSet %s is ready", name))
					return nil
				}

			case watch.Error:
				return fmt.Errorf("error watching StatefulSet %s: %v", name, event.Object)
			}

		case <-timeoutCh:
			return fmt.Errorf("timeout waiting for StatefulSet %s to be ready after %v", name, timeout)
		}
	}
}

// isStatefulSetReady checks if a StatefulSet is ready based on its status
func isStatefulSetReady(statefulSet *appsv1.StatefulSet) bool {
	// Check if the StatefulSet has the desired number of replicas
	if statefulSet.Status.Replicas == 0 {
		return false
	}

	// Check if all replicas are ready
	if statefulSet.Status.ReadyReplicas != *statefulSet.Spec.Replicas {
		return false
	}

	// Check if all replicas are up to date
	if statefulSet.Status.UpdatedReplicas != *statefulSet.Spec.Replicas {
		return false
	}

	return true
}

// configureNetworking sets up service exports for cross-cluster communication
func configureNetworking(ctx *mongo.MigrationContext, c kube.Clusters, resources migration.Resources) error {
	if resources.GetNetworkingTool() == prompt.NetworkingToolSkupper {
		//skupper.CreateSiteConnection(c, ctx.OriginService.Namespace)
	}

	if resources.GetNetworkingTool() == prompt.NetworkingToolLinkerd {
		logger.Info(fmt.Sprintf("Adding linkerd.io/inject=enabled annotation to namespace %s", ctx.Service.Namespace))

		// Fetch the namespace object first
		namespaceInterface, err := c.Target.FetchResource(kube.Namespace, ctx.Service.Namespace, "")
		exit.OnErrorWithMessage(err, "Failed to fetch namespace")
		namespaceObj := namespaceInterface.(*v1core.Namespace)

		// Add the linkerd injection annotation
		err = c.Target.AddAnnotation(namespaceObj, "linkerd.io/inject", "enabled")
		exit.OnErrorWithMessage(err, "Failed to add linkerd inject annotation to namespace")

		// Fetch the namespace object first
		namespaceInterface, err = c.Origin.FetchResource(kube.Namespace, ctx.Service.Namespace, "")
		exit.OnErrorWithMessage(err, "Failed to fetch namespace")
		namespaceObj = namespaceInterface.(*v1core.Namespace)

		// Add the linkerd injection annotation
		err = c.Origin.AddAnnotation(namespaceObj, "linkerd.io/inject", "enabled")
		exit.OnErrorWithMessage(err, "Failed to add linkerd inject annotation to namespace")
	}
	resources.ExportService(c.Origin, ctx.OriginService.Namespace, ctx.OriginService.Name)
	resources.ExportService(c.Target, ctx.TargetService.Namespace, ctx.TargetService.Name)

	time.Sleep(30 * time.Second)
	return nil
}

// updateOriginHosts updates the MongoDB hosts configuration in the origin cluster
func updateOriginHosts(ctx *mongo.MigrationContext, client *mongo.Client) error {
	logger.Debug(fmt.Sprintf("Updated MongoDB hosts for StatefulSet %s: %v", ctx.StatefulSet.Name, ctx.UpdatedHosts))

	return mongo.OverwriteMongoHosts(client, ctx.PrimaryHost, ctx.UpdatedHosts)
}

// addTargetMembersToReplicaSet adds all target members to the MongoDB replica set
func addTargetMembersToReplicaSet(ctx *mongo.MigrationContext, client *mongo.Client) error {
	for _, targetHost := range ctx.TargetHosts {
		if err := mongo.AddMongoMember(client, ctx.PrimaryHost, targetHost); err != nil {
			return fmt.Errorf("failed to add target member %s to origin replica set: %w", targetHost, err)
		}
	}

	for _, targetHost := range ctx.TargetHosts {
		if err := mongo.WaitForMongoMemberSecondary(client, ctx.PrimaryHost, targetHost); err != nil {
			return fmt.Errorf("failed to wait for target member to become SECONDARY: %w", err)
		}
	}

	return nil
}

// transferPrimary promotes target members and demotes origin members
func transferPrimary(ctx *mongo.MigrationContext, client *mongo.Client) error {
	// Promote target members
	for _, targetHost := range ctx.TargetHosts {
		if err := mongo.PromoteMember(client, ctx.PrimaryHost, targetHost); err != nil {
			return fmt.Errorf("failed to promote target member %s: %w", targetHost, err)
		}
	}

	// Demote origin members (current primary steps down)
	for _, originHost := range ctx.UpdatedHosts {
		if err := mongo.DemoteMember(client, ctx.PrimaryHost, originHost); err != nil {
			return fmt.Errorf("failed to demote origin member %s: %w", originHost, err)
		}
	}

	return nil
}

// removeOriginMembers removes all origin members from the MongoDB replica set
func removeOriginMembers(ctx *mongo.MigrationContext, clientOrigin, clientTarget *mongo.Client) error {
	primaryHost := strings.Split(ctx.PrimaryHost, ":")[0]
	currentPrimary, err := mongo.GetPrimaryMongoHost(clientOrigin, primaryHost)
	logger.Info(currentPrimary + " is the current primary host")
	exit.OnErrorWithMessage(err, "failed to get current primary host")

	for _, originHost := range ctx.UpdatedHosts {
		if err := mongo.RemoveMongoMember(clientTarget, currentPrimary, originHost); err != nil {
			return fmt.Errorf("failed to remove member %s from replicaSet: %w", originHost, err)
		}
	}
	return nil
}

func deployMongoSyncer(c kube.Cluster, originURI, targetURI string) {
	config := map[string]string{
		"MONGOSYNC_SOURCE": originURI,
		"MONGOSYNC_TARGET": targetURI,
	}

	configMap := &v1core.ConfigMap{

		ObjectMeta: metav1.ObjectMeta{
			Name:      "mongosyncer-config",
			Namespace: "default",
		},
		Data: config,
	}

	err := CreateResourceIfNotExists(c, kube.ConfigMap, configMap.Namespace, configMap)
	exit.OnErrorWithMessage(err, "Failed to create MongoSyncer ConfigMap")

	err = c.CreateResourcesFromURL(constants.MongoSyncerURL, "default")
	exit.OnErrorWithMessage(err, "Failed to deploy MongoSyncer")
}

func waitForJobCompletion(c kube.Cluster, namespace, jobName string) {
	timeout := time.After(10 * time.Minute)
	tick := time.Tick(5 * time.Second)

	for {
		select {
		case <-timeout:
			exit.OnErrorWithMessage(fmt.Errorf("timeout waiting for job %s in namespace %s to complete", jobName, namespace),
				fmt.Sprintf("Job %s in namespace %s did not complete within 10 minutes", jobName, namespace))
		case <-tick:
			job, err := c.Clientset.BatchV1().Jobs(namespace).Get(context.TODO(), jobName, metav1.GetOptions{})
			exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to get job %s in namespace %s", jobName, namespace))
			if job.Status.Succeeded > 0 {
				logger.Debug(fmt.Sprintf("Job %s in namespace %s has completed", jobName, namespace))
				return
			}
			logger.Debug(fmt.Sprintf("Job %s in namespace %s not completed yet, waiting...", jobName, namespace))
		}
	}
}

func getCredentialsFromStatefulSet(c kube.Cluster, sts appsv1.StatefulSet, db *postgres.DatabaseInstance) {
	for _, env := range sts.Spec.Template.Spec.Containers[0].Env {
		switch env.Name {
		case "MONGO_INITDB_ROOT_USERNAME":
			if env.Value != "" {
				db.Username = env.Value
			} else if env.ValueFrom != nil {
				if env.ValueFrom.ConfigMapKeyRef != nil {
					db.UserLocationType = "ConfigMap"
					db.UserLocation = env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name
					db.UserKey = env.ValueFrom.ConfigMapKeyRef.Key
				} else if env.ValueFrom.SecretKeyRef != nil {
					db.UserLocationType = "Secret"
					db.UserLocation = env.ValueFrom.SecretKeyRef.LocalObjectReference.Name
					db.UserKey = env.ValueFrom.SecretKeyRef.Key
				}
			}
		case "MONGO_INITDB_ROOT_PASSWORD":
			if env.Value != "" {
				db.Password = env.Value
			} else if env.ValueFrom != nil {
				if env.ValueFrom.ConfigMapKeyRef != nil {
					db.PasswordLocationType = "ConfigMap"
					db.PasswordLocation = env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name
					db.PasswordKey = env.ValueFrom.ConfigMapKeyRef.Key
				} else if env.ValueFrom.SecretKeyRef != nil {
					db.PasswordLocationType = "Secret"
					db.PasswordLocation = env.ValueFrom.SecretKeyRef.LocalObjectReference.Name
					db.PasswordKey = env.ValueFrom.SecretKeyRef.Key
				}
			}
		}
	}
	logger.Info(fmt.Sprintf("Extracted credentials for StatefulSet %s: Username=%s, Password=%s, PasswordLocationType=%s, PasswordLocation=%s, PasswordKey=%s",
		sts.Name, db.Username, db.Password, db.PasswordLocationType, db.PasswordLocation, db.PasswordKey))
}

func restoreMongoDBMemberCount(c kube.Cluster, statefulsetName, statefulsetNamespace string, memberCount int) error {
	logger.Debug(fmt.Sprintf("Restoring MongoDB cluster %s member count to %d", statefulsetName, statefulsetNamespace))

	statefulsetObj, err := c.FetchResource(kube.StatefulSet, statefulsetName, statefulsetNamespace)
	exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to fetch StatefulSet %s in namespace %s", statefulsetName, statefulsetNamespace))

	statefulset := statefulsetObj.(*appsv1.StatefulSet)
	if statefulset == nil {
		return fmt.Errorf("StatefulSet %s not found in namespace %s", statefulsetName, statefulsetNamespace)
	}

	statefulset.Spec.Replicas = &[]int32{int32(memberCount)}[0]

	err = c.UpdateResource(kube.StatefulSet, statefulsetName, statefulsetNamespace, statefulset)
	exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to update StatefulSet %s in namespace %s", statefulsetName, statefulsetNamespace))

	return nil
}
