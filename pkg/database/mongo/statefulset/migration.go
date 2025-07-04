package statefulset

import (
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"clustershift/internal/migration"
	"clustershift/internal/prompt"
	"clustershift/pkg/skupper"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
)

// Migrate migrates MongoDB StatefulSets from origin to target cluster
func Migrate(c kube.Clusters, resources migration.Resources) {
	logger.Info("Migrating MongoDBs")

	statefulSets := scanExistingDatabases(c.Origin)
	if len(statefulSets) == 0 {
		logger.Info("No existing MongoDBs found, skipping migration")
		return
	}

	mongoClientOrigin := NewMongoClient(c.Origin, "default")
	mongoClientTarget := NewMongoClient(c.Target, "default")

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
func prepareMigrationContext(statefulSet appsv1.StatefulSet, c kube.Clusters, resources migration.Resources, mongoClientOrigin *MongoClient) (*MigrationContext, error) {
	service, err := getServiceForStatefulSet(statefulSet, c.Origin)
	if err != nil {
		return nil, fmt.Errorf("failed to get service for StatefulSet %s: %w", statefulSet.Name, err)
	}

	primaryHost, err := GetPrimaryMongoHost(mongoClientOrigin, service.Name+".svc."+service.Namespace+".cluster.local")
	if err != nil {
		return nil, fmt.Errorf("failed to get primary MongoDB host for StatefulSet %s: %w", statefulSet.Name, err)
	}

	originHosts, err := GetMongoHosts(mongoClientOrigin, service.Name+".svc."+service.Namespace+".cluster.local")
	if err != nil {
		return nil, fmt.Errorf("failed to get MongoDB hosts for StatefulSet %s: %w", statefulSet.Name, err)
	}

	logger.Debug(fmt.Sprintf("MongoDB hosts for StatefulSet %s: %v", statefulSet.Name, originHosts))

	return &MigrationContext{
		StatefulSet:  statefulSet,
		Service:      service,
		PrimaryHost:  primaryHost,
		OriginHosts:  originHosts,
		UpdatedHosts: UpdateMongoHosts(originHosts, resources, service, c.Origin),
		TargetHosts:  UpdateMongoHosts(originHosts, resources, service, c.Target),
	}, nil
}

// migrateStatefulSet performs the complete migration of a MongoDB StatefulSet
func migrateStatefulSet(ctx *MigrationContext, c kube.Clusters, resources migration.Resources, mongoClientOrigin, mongoClientTarget *MongoClient) error {
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

	if err := waitForTargetPrimaryElection(mongoClientOrigin, ctx); err != nil {
		return fmt.Errorf("failed to wait for new primary election: %w", err)
	}

	if err := removeOriginMembers(ctx, mongoClientOrigin, mongoClientTarget); err != nil {
		return fmt.Errorf("failed to remove origin members: %w", err)
	}

	logger.Info(fmt.Sprintf("Successfully migrated MongoDB StatefulSet %s", ctx.StatefulSet.Name))
	return nil
}

// configureNetworking sets up service exports for cross-cluster communication
func configureNetworking(ctx *MigrationContext, c kube.Clusters, resources migration.Resources) error {
	if resources.GetNetworkingTool() == prompt.NetworkingToolSkupper {
		skupper.CreateSiteConnection(c, ctx.Service.Namespace)
	}

	resources.ExportService(c.Origin, ctx.Service.Namespace, ctx.Service.Name)
	resources.ExportService(c.Target, ctx.Service.Namespace, ctx.Service.Name)

	return nil
}

// updateOriginHosts updates the MongoDB hosts configuration in the origin cluster
func updateOriginHosts(ctx *MigrationContext, client *MongoClient) error {
	logger.Debug(fmt.Sprintf("Updated MongoDB hosts for StatefulSet %s: %v", ctx.StatefulSet.Name, ctx.UpdatedHosts))

	return overwriteMongoHosts(client, ctx.PrimaryHost, ctx.UpdatedHosts)
}

// addTargetMembersToReplicaSet adds all target members to the MongoDB replica set
func addTargetMembersToReplicaSet(ctx *MigrationContext, client *MongoClient) error {
	for _, targetHost := range ctx.TargetHosts {
		if err := addMongoMember(client, ctx.PrimaryHost, targetHost); err != nil {
			return fmt.Errorf("failed to add target member %s to origin replica set: %w", targetHost, err)
		}
	}

	for _, targetHost := range ctx.TargetHosts {
		if err := waitForMongoMemberSecondary(client, ctx.PrimaryHost, targetHost); err != nil {
			return fmt.Errorf("failed to wait for target member to become SECONDARY: %w", err)
		}
	}

	return nil
}

// transferPrimary promotes target members and demotes origin members
func transferPrimary(ctx *MigrationContext, client *MongoClient) error {
	// Promote target members
	for _, targetHost := range ctx.TargetHosts {
		if err := promoteMember(client, ctx.PrimaryHost, targetHost); err != nil {
			return fmt.Errorf("failed to promote target member %s: %w", targetHost, err)
		}
	}

	// Demote origin members (current primary steps down)
	for _, originHost := range ctx.UpdatedHosts {
		if err := demoteMember(client, ctx.PrimaryHost, originHost); err != nil {
			return fmt.Errorf("failed to demote origin member %s: %w", originHost, err)
		}
	}

	return nil
}

// removeOriginMembers removes all origin members from the MongoDB replica set
func removeOriginMembers(ctx *MigrationContext, clientOrigin, clientTarget *MongoClient) error {
	currentPrimary, err := GetPrimaryMongoHost(clientOrigin, ctx.PrimaryHost)
	exit.OnErrorWithMessage(err, "failed to get current primary host")

	for _, originHost := range ctx.UpdatedHosts {
		if err := removeMongoMember(clientTarget, currentPrimary, originHost); err != nil {
			return fmt.Errorf("failed to remove member %s from replicaSet: %w", originHost, err)
		}
	}
	return nil
}
