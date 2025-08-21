package postgres

import (
	"bytes"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"clustershift/internal/migration"
	"clustershift/internal/prompt"
	"clustershift/pkg/skupper"
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"strings"
	"time"
)

func Migrate(c kube.Clusters, resources migration.Resources) {
	logger.Info("Migrating PostgreSQL databases")

	statefulSet := scanExistingDatabases(c.Origin)
	if len(statefulSet) == 0 {
		logger.Info("No existing PostgreSQL databases found, skipping migration")
		return
	}

	for _, sts := range statefulSet {
		db := DatabaseInstance{}
		db.StatefulsetName = sts.Name
		db.Namespace = sts.Namespace

		serviceName, err := getServiceNameForStatefulSet(sts, c.Origin)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to get service name for StatefulSet %s", sts.Name))
		db.ServiceName = serviceName

		imageName, err := getContainerImageFromStatefulSet(sts)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to get container image from StatefulSet %s", sts.Name))
		db.ImageName = imageName

		getCredentialsFromStatefulSet(c.Origin, sts, &db)

		if resources.GetNetworkingTool() == prompt.NetworkingToolSkupper {
			skupper.CreateSiteConnection(c, db.Namespace)
		}
		resources.ExportService(c.Origin, db.Namespace, db.ServiceName)

		if resources.GetNetworkingTool() == prompt.NetworkingToolLinkerd {
			namespaceInterface, err := c.Target.FetchResource(kube.Namespace, db.Namespace, "")
			exit.OnErrorWithMessage(err, "Failed to fetch namespace")
			namespaceObj := namespaceInterface.(*v1core.Namespace)

			// Check if the namespace already has the linkerd.io/inject annotation
			if namespaceObj.Annotations == nil || namespaceObj.Annotations["linkerd.io/inject"] != "enabled" {
				// Add the linkerd injection annotation
				err = c.Target.AddAnnotation(namespaceObj, "linkerd.io/inject", "enabled")
				exit.OnErrorWithMessage(err, "Failed to add linkerd inject annotation to namespace")
			}
		}

		err = enableReplication(c.Origin, db)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to enable replication for %s", db.StatefulsetName))

		err = copyResources(c, db, resources)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to copy resources for %s", db.StatefulsetName))

		// Wait for replication to be ready (wait up to 10 minutes)
		err = waitForReplicationReady(c, db, 10*time.Minute)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to wait for replication readiness for %s", db.StatefulsetName))

		// Decouple target database from source to make it independent
		err = decoupleTargetFromSource(c, db)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to decouple target database %s from source", db.StatefulsetName))

		logger.Info(fmt.Sprintf("Successfully migrated PostgreSQL database %s", db.StatefulsetName))
	}
}

func scanExistingDatabases(c kube.Cluster) []appsv1.StatefulSet {
	statefulSets, err := c.Clientset.AppsV1().StatefulSets("").List(context.TODO(), metav1.ListOptions{})
	exit.OnErrorWithMessage(err, "Failed to list statefulsets")

	var matches []appsv1.StatefulSet
	for _, sts := range statefulSets.Items {
		for _, container := range sts.Spec.Template.Spec.Containers {
			if strings.Contains(container.Image, "bitnami/postgresql") {
				matches = append(matches, sts)
				break
			}
		}
	}
	return matches
}

func getServiceNameForStatefulSet(sts appsv1.StatefulSet, c kube.Cluster) (string, error) {
	ns := sts.Namespace

	services, err := c.Clientset.CoreV1().Services(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list services: %w", err)
	}

	for _, svc := range services.Items {
		if reflect.DeepEqual(svc.Spec.Selector, sts.Spec.Selector.MatchLabels) {
			return svc.Name, nil
		}
	}

	return "", fmt.Errorf("no matching service found for statefulset %s", sts.Name)
}

func getContainerImageFromStatefulSet(sts appsv1.StatefulSet) (string, error) {
	if len(sts.Spec.Template.Spec.Containers) == 0 {
		return "", fmt.Errorf("no containers found in StatefulSet %s", sts.Name)
	}
	return sts.Spec.Template.Spec.Containers[0].Image, nil
}

func getCredentialsFromStatefulSet(c kube.Cluster, sts appsv1.StatefulSet, db *DatabaseInstance) {
	for _, env := range sts.Spec.Template.Spec.Containers[0].Env {
		switch env.Name {
		case "POSTGRESQL_USERNAME":
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
		case "POSTGRESQL_POSTGRES_PASSWORD":
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

	if db.Username != "" && db.Password != "" {
		return
	}
	if db.UserLocation != "" && db.UserKey != "" && db.PasswordLocation != "" && db.PasswordKey != "" {
		db.Username = "postgres"
		password, err := GetPostgresPasswordFromStatefulSet(c, db.PasswordLocation, db.PasswordKey, sts.Namespace)
		exit.OnErrorWithMessage(err, fmt.Sprintf("Failed to get PostgreSQL password from StatefulSet %s", sts.Name))
		db.Password = password
	}

	logger.Info(fmt.Sprintf("%s, %s", db.Username, db.Password))
}

func GetPostgresPasswordFromStatefulSet(c kube.Cluster, passwordLocation, passwordKey, namespace string) (string, error) {

	secret, err := c.Clientset.CoreV1().Secrets(namespace).Get(context.TODO(), passwordLocation, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get secret %s: %w", passwordLocation, err)
	}

	passwordBytes, ok := secret.Data[passwordKey]
	if !ok {
		return "", fmt.Errorf("postgres-password key not found in secret %s", passwordLocation)
	}

	password := string(passwordBytes)
	return password, nil
}

func createReplicationUserJob(c kube.Cluster, db DatabaseInstance) error {
	var envVars []corev1.EnvVar

	// Username
	if db.Username != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "PGUSER",
			Value: db.Username,
		})
	} else if db.UserLocation != "" && db.UserKey != "" {
		envVar := corev1.EnvVar{Name: "PGUSER"}
		if db.UserLocationType == "Secret" {
			envVar.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: db.UserLocation},
					Key:                  db.UserKey,
				},
			}
		} else if db.UserLocationType == "ConfigMap" {
			envVar.ValueFrom = &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: db.UserLocation},
					Key:                  db.UserKey,
				},
			}
		}
		envVars = append(envVars, envVar)
	}

	// Password
	if db.Password != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "PGPASSWORD",
			Value: db.Password,
		})
	} else if db.PasswordLocation != "" && db.PasswordKey != "" {
		envVar := corev1.EnvVar{Name: "PGPASSWORD"}
		if db.PasswordLocationType == "Secret" {
			envVar.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: db.PasswordLocation},
					Key:                  db.PasswordKey,
				},
			}
		} else if db.PasswordLocationType == "ConfigMap" {
			envVar.ValueFrom = &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: db.PasswordLocation},
					Key:                  db.PasswordKey,
				},
			}
		}
		envVars = append(envVars, envVar)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "create-replication-user",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "psql",
							Image: db.ImageName,
							Env:   envVars,
							Command: []string{
								"psql", "-h", fmt.Sprintf("%s.%s.svc.cluster.local", db.ServiceName, db.Namespace),
								"-U", "postgres", "-d", "postgres",
								"-c", "CREATE ROLE repl_user WITH REPLICATION LOGIN PASSWORD 'replication_password';",
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
			TTLSecondsAfterFinished: func(i int32) *int32 { return &i }(60),
		},
	}

	_, err := c.Clientset.BatchV1().Jobs(db.Namespace).Create(context.TODO(), job, metav1.CreateOptions{})
	return err
}

func copyResources(c kube.Clusters, db DatabaseInstance, resources migration.Resources) error {
	ctx := context.TODO()
	ns := db.Namespace

	// Copy StatefulSet
	sts, err := c.Origin.Clientset.AppsV1().StatefulSets(ns).Get(ctx, db.StatefulsetName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get StatefulSet: %w", err)
	}
	sts.ResourceVersion = ""

	// Add replication environment variables to make target a replica
	if len(sts.Spec.Template.Spec.Containers) > 0 {
		container := &sts.Spec.Template.Spec.Containers[0]

		// Add replication-specific environment variables
		replicationEnvs := []corev1.EnvVar{
			{
				Name:  "POSTGRESQL_REPLICATION_MODE",
				Value: "slave",
			},
			{
				Name:  "POSTGRESQL_MASTER_HOST",
				Value: resources.GetPostgresDNSName(db.ServiceName, db.Namespace),
			},
			{
				Name:  "POSTGRESQL_REPLICATION_USER",
				Value: "repl_user",
			},
			{
				Name:  "POSTGRESQL_REPLICATION_PASSWORD",
				Value: "replication_password",
			},
			{
				Name:  "POSTGRESQL_PGHBA",
				Value: "host replication repl_user 0.0.0.0/0 md5",
			},
		}

		// Add the replication environment variables to the container
		container.Env = append(container.Env, replicationEnvs...)

		logger.Info(fmt.Sprintf("Added replication environment variables to StatefulSet %s", db.StatefulsetName))
	}

	_, err = c.Target.Clientset.AppsV1().StatefulSets(ns).Create(ctx, sts, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create StatefulSet: %w", err)
	}

	// Copy Service
	svc, err := c.Origin.Clientset.CoreV1().Services(ns).Get(ctx, db.ServiceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get Service: %w", err)
	}
	svc.ResourceVersion = ""
	_, err = c.Target.Clientset.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Service: %w", err)
	}

	return nil
}

// TODO Enable replication in source database
func enableReplication(c kube.Cluster, db DatabaseInstance) error {
	var out, errOut bytes.Buffer

	cmds := [][]string{
		{"env", "PGPASSWORD=" + db.Password, "/opt/bitnami/postgresql/bin/psql", "-U", "postgres", "-c", "CREATE ROLE repl_user WITH REPLICATION LOGIN PASSWORD 'replication_password';"},
		{"bash", "-c", "echo 'host replication repl_user 0.0.0.0/0 md5' >> opt/bitnami/postgresql/conf/pg_hba.conf"},
		{"bash", "-c", "echo \"wal_level = replica\" >> opt/bitnami/postgresql/conf/postgresql.conf"},
		{"pg_ctl", "reload", "-D", "/bitnami/postgresql/data"},
	}
	for _, cmd := range cmds {
		err := c.ExecIntoPod(db.Namespace, db.StatefulsetName+"-0", "", cmd, &out, &errOut)
		if err != nil {
			return fmt.Errorf("failed to execute psql command: %w, stderr: %s", err, errOut.String())
		}
		if errOut.Len() > 0 {
			return fmt.Errorf("psql error: %s", errOut.String())
		}
	}

	return nil
}

// waitForReplicationReady waits for the target database to be ready for replication
// by checking if the target PostgreSQL instance is running and can connect to the source
func waitForReplicationReady(c kube.Clusters, db DatabaseInstance, maxWaitTime time.Duration) error {
	logger.Info(fmt.Sprintf("Waiting for replication to be ready for %s", db.StatefulsetName))

	timeout := time.After(maxWaitTime)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for replication to be ready after %v", maxWaitTime)
		case <-ticker.C:
			// Check if target pod is running
			pod, err := c.Target.Clientset.CoreV1().Pods(db.Namespace).Get(context.TODO(), db.StatefulsetName+"-0", metav1.GetOptions{})
			if err != nil {
				logger.Info(fmt.Sprintf("Target pod not found yet: %v", err))
				continue
			}

			if pod.Status.Phase != corev1.PodRunning {
				logger.Info(fmt.Sprintf("Target pod not running yet, current phase: %s", pod.Status.Phase))
				continue
			}

			// Check if all containers are ready
			allReady := true
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if !containerStatus.Ready {
					allReady = false
					break
				}
			}

			if !allReady {
				logger.Info("Target pod containers not ready yet")
				continue
			}

			// Test database connectivity on target
			var out, errOut bytes.Buffer
			testCmd := []string{"env", "PGPASSWORD=" + db.Password, "/opt/bitnami/postgresql/bin/psql", "-U", "postgres", "-c", "SELECT 1;"}
			err = c.Target.ExecIntoPod(db.Namespace, db.StatefulsetName+"-0", "", testCmd, &out, &errOut)
			if err != nil {
				logger.Info(fmt.Sprintf("Target database not ready yet: %v", err))
				continue
			}

			// Check replication status by testing if we can connect to source from target
			sourceHost := fmt.Sprintf("%s.%s.svc.cluster.local", db.ServiceName, db.Namespace)
			replicationTestCmd := []string{"env", "PGPASSWORD=replication_password", "/opt/bitnami/postgresql/bin/psql",
				"-h", sourceHost, "-U", "repl_user", "-d", "postgres", "-c", "SELECT pg_is_in_recovery();"}

			out.Reset()
			errOut.Reset()
			err = c.Target.ExecIntoPod(db.Namespace, db.StatefulsetName+"-0", "", replicationTestCmd, &out, &errOut)
			if err != nil {
				logger.Info(fmt.Sprintf("Replication connection test failed: %v", err))
				continue
			}

			logger.Info("Replication is ready and target database is operational")
			return nil
		}
	}
}

// decoupleTargetFromSource promotes the target database to be a standalone master
// by stopping replication and making it independent from the source
func decoupleTargetFromSource(c kube.Clusters, db DatabaseInstance) error {
	logger.Info(fmt.Sprintf("Decoupling target database %s from source", db.StatefulsetName))

	var out, errOut bytes.Buffer

	// First, check if the target is in recovery mode (i.e., it's a replica)
	checkRecoveryCmd := []string{"env", "PGPASSWORD=" + db.Password, "/opt/bitnami/postgresql/bin/psql",
		"-U", "postgres", "-c", "SELECT pg_is_in_recovery();"}

	err := c.Target.ExecIntoPod(db.Namespace, db.StatefulsetName+"-0", "", checkRecoveryCmd, &out, &errOut)
	if err != nil {
		return fmt.Errorf("failed to check recovery status: %w, stderr: %s", err, errOut.String())
	}

	// If the database is in recovery mode, promote it to master
	if strings.Contains(out.String(), "t") { // 't' means true, indicating it's in recovery
		logger.Info("Target database is in recovery mode, promoting to master")

		// Promote the replica to master using pg_promote()
		out.Reset()
		errOut.Reset()
		promoteCmd := []string{"env", "PGPASSWORD=" + db.Password, "/opt/bitnami/postgresql/bin/psql",
			"-U", "postgres", "-c", "SELECT pg_promote();"}

		err = c.Target.ExecIntoPod(db.Namespace, db.StatefulsetName+"-0", "", promoteCmd, &out, &errOut)
		if err != nil {
			return fmt.Errorf("failed to promote replica to master: %w, stderr: %s", err, errOut.String())
		}

		// Wait a moment for promotion to complete
		time.Sleep(5 * time.Second)

		// Verify promotion was successful
		out.Reset()
		errOut.Reset()
		err = c.Target.ExecIntoPod(db.Namespace, db.StatefulsetName+"-0", "", checkRecoveryCmd, &out, &errOut)
		if err != nil {
			return fmt.Errorf("failed to verify promotion: %w, stderr: %s", err, errOut.String())
		}

		if strings.Contains(out.String(), "f") { // 'f' means false, indicating it's no longer in recovery
			logger.Info("Successfully promoted target database to master")
		} else {
			return fmt.Errorf("promotion failed - database is still in recovery mode")
		}
	} else {
		logger.Info("Target database is already in master mode")
	}

	logger.Info("Target database successfully decoupled and is now operating as an independent master")
	return nil
}

func renameTargetService(c kube.Cluster, originalName, namespace string, toTemporary bool) error {
	svc, err := c.Clientset.CoreV1().Services(namespace).Get(context.TODO(), originalName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get service %s: %w", originalName, err)
	}

	// Rename the service by adding a suffix or prefix
	newName := originalName
	if toTemporary {
		newName += "-temp"
	} else {
		newName = strings.TrimSuffix(originalName, "-temp")
	}

	svc.Name = newName
	svc.ResourceVersion = ""

	_, err = c.Clientset.CoreV1().Services(namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create renamed service %s: %w", newName, err)
	}

	// Optionally, delete the old service if renaming to temporary
	if toTemporary {
		err = c.Clientset.CoreV1().Services(namespace).Delete(context.TODO(), originalName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete original service %s after renaming: %w", originalName, err)
		}
	}

	return nil
}
