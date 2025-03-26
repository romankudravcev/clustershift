// This code is copied from "https://github.com/linkerd/linkerd2" repository
// and has been modified for our specific use case.

package linkerd

import (
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/linkerd/linkerd2/controller/gen/apis/link/v1alpha1"
	"github.com/linkerd/linkerd2/pkg/charts/linkerd2"
	"github.com/linkerd/linkerd2/pkg/multicluster"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"strings"
)

func createKubeconfig(fromCluster kube.Cluster, opts *linkOptions) []byte {
	// fetch service account
	serviceAccountInterface, err := fromCluster.FetchResource(kube.ServiceAccount, opts.serviceAccountName, opts.namespace)
	exit.OnErrorWithMessage(err, "Service account not found")
	sa := serviceAccountInterface.(*corev1.ServiceAccount)

	// fetch secrets
	listOpts := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("type=%s", corev1.SecretTypeServiceAccountToken),
	}
	secrets, err := fromCluster.Clientset.CoreV1().Secrets(opts.namespace).List(context.TODO(), listOpts)
	exit.OnErrorWithMessage(err, "Secrets not found")

	// extract token
	token, err := extractSAToken(secrets.Items, sa.Name)
	exit.OnErrorWithMessage(err, "Error extracting token")

	config := fromCluster.Config

	configContext, ok := config.Contexts[config.CurrentContext]
	if !ok {
		exit.OnErrorWithMessage(errors.New("no Context"), "Context not found")
	}

	configContext.AuthInfo = opts.serviceAccountName
	config.Contexts = map[string]*api.Context{
		config.CurrentContext: configContext,
	}
	config.AuthInfos = map[string]*api.AuthInfo{
		opts.serviceAccountName: {
			Token: token,
		},
	}

	cluster := config.Clusters[configContext.Cluster]
	cluster.Server = opts.apiServerAddress

	config.Clusters = map[string]*api.Cluster{
		configContext.Cluster: cluster,
	}

	kubeconfig, err := clientcmd.Write(*config)
	exit.OnErrorWithMessage(err, "Error generating kubeconfig")

	return kubeconfig
}

func createSecrets(toCluster kube.Cluster, configMapValue linkerd2.Values, opts *linkOptions, kubeconfig []byte) {
	creds := corev1.Secret{
		Type:     "mirror.linkerd.io/remote-kubeconfig",
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cluster-credentials-%s", opts.clusterName),
			Namespace: opts.namespace,
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfig,
		},
	}
	_, err := toCluster.Clientset.CoreV1().Secrets(creds.Namespace).Create(context.TODO(), &creds, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			logger.Warning(fmt.Sprintf("Secret %s already exists in namespace %s, updating it", creds.Name, creds.Namespace), err)
			_, err = toCluster.Clientset.CoreV1().Secrets(creds.Namespace).Update(context.TODO(), &creds, metav1.UpdateOptions{})
			if err != nil {
				exit.OnErrorWithMessage(err, "Error updating secret")
			}
		} else {
			exit.OnErrorWithMessage(err, "Error creating secret")
		}
	}

	destinationCreds := corev1.Secret{
		Type:     "mirror.linkerd.io/remote-kubeconfig",
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cluster-credentials-%s", opts.clusterName),
			Namespace: "linkerd",
			Labels: map[string]string{
				clusterNameLabel: opts.clusterName,
			},
			Annotations: map[string]string{
				trustDomainAnnotation:   configMapValue.IdentityTrustDomain,
				clusterDomainAnnotation: configMapValue.ClusterDomain,
			},
		},
		Data: map[string][]byte{
			"kubeconfig": kubeconfig,
		},
	}
	_, err = toCluster.Clientset.CoreV1().Secrets(destinationCreds.Namespace).Create(context.TODO(), &destinationCreds, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			logger.Warning(fmt.Sprintf("Secret %s already exists in namespace %s, updating it", destinationCreds.Name, destinationCreds.Namespace), err)
			_, err = toCluster.Clientset.CoreV1().Secrets(destinationCreds.Namespace).Update(context.TODO(), &destinationCreds, metav1.UpdateOptions{})
			if err != nil {
				exit.OnErrorWithMessage(err, "Error updating secret")
			}
		} else {
			exit.OnErrorWithMessage(err, "Error creating secret")
		}
	}
}

func createLink(fromCluster kube.Cluster, toCluster kube.Cluster, opts *linkOptions) {
	remoteDiscoverySelector, err := metav1.ParseToLabelSelector(opts.remoteDiscoverySelector)
	exit.OnErrorWithMessage(err, "Error parsing remote discovery selector")
	federatedServiceSelector, err := metav1.ParseToLabelSelector(opts.federatedServiceSelector)
	exit.OnErrorWithMessage(err, "Error parsing federated service selector")

	link := v1alpha1.Link{
		TypeMeta: metav1.TypeMeta{Kind: "Link", APIVersion: "multicluster.linkerd.io/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      opts.clusterName,
			Namespace: opts.namespace,
			Annotations: map[string]string{
				"linkerd.io/created-by": "clustershift",
			},
		},
		Spec: v1alpha1.LinkSpec{
			TargetClusterName:             opts.clusterName,
			TargetClusterDomain:           "cluster.local",
			TargetClusterLinkerdNamespace: "linkerd",
			ClusterCredentialsSecret:      fmt.Sprintf("cluster-credentials-%s", opts.clusterName),
			RemoteDiscoverySelector:       remoteDiscoverySelector,
			FederatedServiceSelector:      federatedServiceSelector,
		},
	}

	// If there is a gateway in the exporting cluster, populate Link
	// resource with gateway information
	if opts.enableGateway {
		logger.Info(fmt.Sprintf("Try fetching gateway service %s in namespace %s", opts.gatewayName, opts.gatewayNamespace))
		gatewayInterface, err := fromCluster.FetchResource(kube.Service, opts.gatewayName, opts.gatewayNamespace)
		gateway := gatewayInterface.(*corev1.Service)
		exit.OnErrorWithMessage(err, "Gateway not found")

		var gwAddresses []string
		for _, ingress := range gateway.Status.LoadBalancer.Ingress {
			addr := ingress.IP
			if addr == "" {
				addr = ingress.Hostname
			}
			if addr == "" {
				continue
			}
			gwAddresses = append(gwAddresses, addr)
		}

		if opts.gatewayAddresses != "" {
			link.Spec.GatewayAddress = opts.gatewayAddresses
		} else if len(gwAddresses) > 0 {
			link.Spec.GatewayAddress = strings.Join(gwAddresses, ",")
		} else {
			exit.OnErrorWithMessage(fmt.Errorf("no gateway addresses found"), "Gateway not found")
		}

		gatewayIdentity, ok := gateway.Annotations["mirror.linkerd.io/gateway-identity"]
		if !ok || gatewayIdentity == "" {
			exit.OnErrorWithMessage(fmt.Errorf("no gateway identity found"), "Gateway not found")
		}
		link.Spec.GatewayIdentity = gatewayIdentity

		probeSpec, err := multicluster.ExtractProbeSpec(gateway)
		exit.OnErrorWithMessage(err, "Error extracting probe spec")
		link.Spec.ProbeSpec = probeSpec

		gatewayPort, err := extractGatewayPort(gateway)
		exit.OnErrorWithMessage(err, "Error extracting gateway port")

		// Override with user provided gateway port if present
		if opts.gatewayPort != 0 {
			gatewayPort = opts.gatewayPort
		}
		link.Spec.GatewayPort = fmt.Sprintf("%d", gatewayPort)

		link.Spec.Selector, err = metav1.ParseToLabelSelector(opts.selector)
		exit.OnErrorWithMessage(err, "Error parsing selector")
	}

	linkBytes, err := json.Marshal(link)
	exit.OnErrorWithMessage(err, "Error marshalling Link to JSON")

	var linkMap map[string]interface{}
	err = json.Unmarshal(linkBytes, &linkMap)
	exit.OnErrorWithMessage(err, "error unmarshalling Link JSON to map")

	err = toCluster.CreateCustomResource(link.Namespace, linkMap)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			logger.Warning("Link already exists", err)
		} else {
			exit.OnErrorWithMessage(err, "Error creating Link")
		}
	}
}
