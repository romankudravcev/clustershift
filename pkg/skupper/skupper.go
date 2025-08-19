package skupper

import (
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"fmt"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"time"
)

func Install(c kube.Clusters) {
	logger.Info("Installing Skupper")

	// Deploy Site Controller
	CreateSiteController(c.Origin)
	CreateSiteController(c.Target)
}

func CreateSiteConnection(c kube.Clusters, siteNamespace string) {
	logger.Info("Creating Site Connection on Namespace: " + siteNamespace)

	// Create Site
	CreateSite(c.Origin, c.Origin.Name+"-"+siteNamespace, siteNamespace)
	CreateSite(c.Target, c.Target.Name+"-"+siteNamespace, siteNamespace)

	// Link target to origin
	CreateConnectionToken(c.Origin, "clustershift-token-"+c.Origin.Name+"-"+siteNamespace, siteNamespace)
	ExtractConnectionToken(c.Origin, c.Target, "clustershift-token-"+c.Origin.Name+"-"+siteNamespace, siteNamespace)

	// Link origin to target
	CreateConnectionToken(c.Target, "clustershift-token-"+c.Target.Name+"-"+siteNamespace, siteNamespace)
	ExtractConnectionToken(c.Target, c.Origin, "clustershift-token-"+c.Target.Name+"-"+siteNamespace, siteNamespace)
}

func CreateSiteController(c kube.Cluster) {
	logger.Info("Deploying Site Controller")

	c.CreateNewNamespace("skupper-site-controller")
	err := c.CreateResourcesFromURL(constants.SkupperSiteControllerURL, "skupper-site-controller")
	if err != nil {
		if k8serrors.IsAlreadyExists(err) || strings.Contains(err.Error(), "already exists") {
			logger.Info("Skupper site controller resources already exist, continuing...")
			return
		} else {
			exit.OnErrorWithMessage(err, "Failed to create resources from URL")
		}
	}

	err = kube.WaitForPodsReadyByLabel(c, "application=skupper-site-controller", "skupper-site-controller", 90*time.Second)
	exit.OnErrorWithMessage(err, "Failed to wait for Site Controller pods to be ready")
}

func CreateSite(c kube.Cluster, name, namespace string) {
	logger.Info("Creating Site")

	data := map[string]string{
		"name": name,
	}
	c.CreateConfigmap("skupper-site", namespace, data)

	err := kube.WaitForPodsReadyByLabel(c, "application=skupper-router", namespace, 90*time.Second)
	exit.OnErrorWithMessage(err, "Failed to wait for Skupper pods to be ready")

	err = kube.WaitForPodsReadyByLabel(c, "app.kubernetes.io/name=skupper-service-controller", namespace, 90*time.Second)
	exit.OnErrorWithMessage(err, "Failed to wait for Skupper pods to be ready")
}

func CreateConnectionToken(c kube.Cluster, name, namespace string) {
	logger.Info("Creating Connection Token")

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"skupper.io/type": "connection-token-request",
			},
			Annotations: map[string]string{
				"skupper.io/cost": "2",
			},
		},
	}

	err := c.CreateResource(kube.Secret, namespace, secret)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) || strings.Contains(err.Error(), "already exists") {
			logger.Info("Secret already existing...")
			return
		} else {
			exit.OnErrorWithMessage(err, "Failed to create secret")
		}
	}

	// Wait for the controller to populate the secret with data
	logger.Info("Waiting for token to be populated with data")
	timeout := 120 * time.Second
	pollInterval := 5 * time.Second
	endTime := time.Now().Add(timeout)

	for time.Now().Before(endTime) {
		secretInterface, err := c.FetchResource(kube.Secret, name, namespace)
		if err == nil {
			secret, ok := secretInterface.(*v1.Secret)
			if ok && len(secret.Data) > 0 {
				logger.Info("Token successfully populated with data")
				return
			}
		}
		time.Sleep(pollInterval)
	}

	exit.OnErrorWithMessage(fmt.Errorf("timeout waiting for secret data"), "Secret data was not populated within timeout period")
}

func ExtractConnectionToken(from kube.Cluster, to kube.Cluster, name, namespace string) {
	logger.Info("Extracting Connection Token")
	secretInterface, err := from.FetchResource(kube.Secret, name, namespace)
	exit.OnErrorWithMessage(err, "Failed to fetch secret")
	cleanedSecretInterface := kube.CleanResourceForCreation(secretInterface)
	secret := cleanedSecretInterface.(*v1.Secret)
	err = to.CreateResource(kube.Secret, namespace, secret)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) || strings.Contains(err.Error(), "already exists") {
			logger.Info("Secret already existing...")
			return
		} else {
			exit.OnErrorWithMessage(err, "Failed to create secret")
		}
	}
}
