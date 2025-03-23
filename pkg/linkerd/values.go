// This code is copied from "https://github.com/linkerd/linkerd2" repository
// and has been modified for our specific use case.

package linkerd

import (
	"fmt"
	multicluster "github.com/linkerd/linkerd2/multicluster/values"
	"github.com/linkerd/linkerd2/pkg/charts/linkerd2"
	"github.com/linkerd/linkerd2/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	clusterNameLabel        = "multicluster.linkerd.io/cluster-name"
	trustDomainAnnotation   = "multicluster.linkerd.io/trust-domain"
	clusterDomainAnnotation = "multicluster.linkerd.io/cluster-domain"
)

type (
	Values struct {
		CliVersion                     string   `yaml:"cliVersion"`
		ControllerImage                string   `yaml:"controllerImage"`
		ControllerImageVersion         string   `yaml:"controllerImageVersion"`
		Gateway                        *Gateway `yaml:"gateway"`
		IdentityTrustDomain            string   `yaml:"identityTrustDomain"`
		LinkerdNamespace               string   `yaml:"linkerdNamespace"`
		LinkerdVersion                 string   `yaml:"linkerdVersion"`
		ProxyOutboundPort              uint32   `yaml:"proxyOutboundPort"`
		ServiceMirror                  bool     `yaml:"serviceMirror"`
		LogLevel                       string   `yaml:"logLevel"`
		LogFormat                      string   `yaml:"logFormat"`
		ServiceMirrorRetryLimit        uint32   `yaml:"serviceMirrorRetryLimit"`
		ServiceMirrorUID               int64    `yaml:"serviceMirrorUID"`
		ServiceMirrorGID               int64    `yaml:"serviceMirrorGID"`
		Replicas                       uint32   `yaml:"replicas"`
		RemoteMirrorServiceAccount     bool     `yaml:"remoteMirrorServiceAccount"`
		RemoteMirrorServiceAccountName string   `yaml:"remoteMirrorServiceAccountName"`
		TargetClusterName              string   `yaml:"targetClusterName"`
		EnablePodAntiAffinity          bool     `yaml:"enablePodAntiAffinity"`
		RevisionHistoryLimit           uint32   `yaml:"revisionHistoryLimit"`

		ServiceMirrorAdditionalEnv   []corev1.EnvVar `yaml:"serviceMirrorAdditionalEnv"`
		ServiceMirrorExperimentalEnv []corev1.EnvVar `yaml:"serviceMirrorExperimentalEnv"`

		LocalServiceMirror *LocalServiceMirror `json:"localServiceMirror"`
	}

	// Gateway contains all options related to the Gateway Service
	Gateway struct {
		Enabled            bool              `yaml:"enabled"`
		Replicas           uint32            `yaml:"replicas"`
		Name               string            `yaml:"name"`
		Port               uint32            `yaml:"port"`
		NodePort           uint32            `yaml:"nodePort"`
		ServiceType        string            `yaml:"serviceType"`
		Probe              *Probe            `yaml:"probe"`
		ServiceAnnotations map[string]string `yaml:"serviceAnnotations"`
		LoadBalancerIP     string            `yaml:"loadBalancerIP"`
		PauseImage         string            `yaml:"pauseImage"`
		UID                int64             `yaml:"UID"`
		GID                int64             `yaml:"GID"`
	}

	// Probe contains all options for the Probe Service
	Probe struct {
		FailureThreshold uint32 `yaml:"failureThreshold"`
		Path             string `yaml:"path"`
		Port             uint32 `yaml:"port"`
		NodePort         uint32 `yaml:"nodePort"`
		Seconds          uint32 `yaml:"seconds"`
		Timeout          string `yaml:"timeout"`
	}

	LocalServiceMirror struct {
		ServiceMirrorRetryLimit  uint32          `yaml:"serviceMirrorRetryLimit"`
		FederatedServiceSelector string          `yaml:"federatedServiceSelector"`
		Replias                  uint32          `yaml:"replicas"`
		Image                    *linkerd2.Image `yaml:"image"`
		LogLevel                 string          `yaml:"logLevel"`
		LogFormat                string          `yaml:"logFormat"`
		EnablePprof              bool            `yaml:"enablePprof"`
		UID                      int64           `yaml:"UID"`
		GID                      int64           `yaml:"GID"`
	}

	linkOptions struct {
		namespace                string
		clusterName              string
		apiServerAddress         string
		serviceAccountName       string
		gatewayName              string
		gatewayNamespace         string
		serviceMirrorRetryLimit  uint32
		logLevel                 string
		logFormat                string
		controlPlaneVersion      string
		dockerRegistry           string
		selector                 string
		remoteDiscoverySelector  string
		federatedServiceSelector string
		gatewayAddresses         string
		gatewayPort              uint32
		ha                       bool
		enableGateway            bool
		output                   string
	}

	Link struct {
		// TypeMeta is the metadata for the resource, like kind and apiversion
		metav1.TypeMeta `json:",inline"`

		// ObjectMeta contains the metadata for the particular object, including
		// things like...
		//  - name
		//  - namespace
		//  - self link
		//  - labels
		//  - ... etc ...
		metav1.ObjectMeta `json:"metadata,omitempty"`

		// Spec is the custom resource spec
		Spec LinkSpec `json:"spec"`

		// Status defines the current state of a Link
		Status LinkStatus `json:"status,omitempty"`
	}

	LinkSpec struct {
		TargetClusterName             string                `json:"targetClusterName,omitempty"`
		TargetClusterDomain           string                `json:"targetClusterDomain,omitempty"`
		TargetClusterLinkerdNamespace string                `json:"targetClusterLinkerdNamespace,omitempty"`
		ClusterCredentialsSecret      string                `json:"clusterCredentialsSecret,omitempty"`
		GatewayAddress                string                `json:"gatewayAddress,omitempty"`
		GatewayPort                   string                `json:"gatewayPort,omitempty"`
		GatewayIdentity               string                `json:"gatewayIdentity,omitempty"`
		ProbeSpec                     ProbeSpec             `json:"probeSpec,omitempty"`
		Selector                      *metav1.LabelSelector `json:"selector,omitempty"`
		RemoteDiscoverySelector       *metav1.LabelSelector `json:"remoteDiscoverySelector,omitempty"`
		FederatedServiceSelector      *metav1.LabelSelector `json:"federatedServiceSelector,omitempty"`
	}

	// ProbeSpec for gateway health probe
	ProbeSpec struct {
		Path             string `json:"path,omitempty"`
		Port             string `json:"port,omitempty"`
		Period           string `json:"period,omitempty"`
		Timeout          string `json:"timeout,omitempty"`
		FailureThreshold string `json:"failureThreshold,omitempty"`
	}

	LinkStatus struct {
		// +optional
		MirrorServices []ServiceStatus `json:"mirrorServices,omitempty"`
		// +optional
		FederatedServices []ServiceStatus `json:"federatedServices,omitempty"`
	}

	ServiceStatus struct {
		Conditions     []LinkCondition `json:"conditions,omitempty"`
		ControllerName string          `json:"controllerName,omitempty"`
		RemoteRef      ObjectRef       `json:"remoteRef,omitempty"`
	}

	// LinkCondition represents the service state of an ExternalWorkload
	LinkCondition struct {
		// Type of the condition
		Type string `json:"type"`
		// Status of the condition.
		// Can be True, False, Unknown
		Status metav1.ConditionStatus `json:"status"`
		// Last time an ExternalWorkload was probed for a condition.
		// +optional
		LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
		// Last time a condition transitioned from one status to another.
		// +optional
		LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
		// Unique one word reason in CamelCase that describes the reason for a
		// transition.
		// +optional
		Reason string `json:"reason,omitempty"`
		// Human readable message that describes details about last transition.
		// +optional
		Message string `json:"message"`
		// LocalRef is a reference to the local mirror or federated service.
		LocalRef ObjectRef `json:"localRef,omitempty"`
	}

	ObjectRef struct {
		Group     string `json:"group,omitempty"`
		Kind      string `json:"kind,omitempty"`
		Name      string `json:"name,omitempty"`
		Namespace string `json:"namespace,omitempty"`
	}

	// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

	// LinkList is a list of LinkList resources.
	LinkList struct {
		metav1.TypeMeta `json:",inline"`
		metav1.ListMeta `json:"metadata"`

		Items []Link `json:"items"`
	}
)

func newLinkOptionsWithDefault() (*linkOptions, error) {
	defaults, err := multicluster.NewLinkValues()
	if err != nil {
		return nil, err
	}

	return &linkOptions{
		controlPlaneVersion:      version.Version,
		namespace:                "linkerd-multicluster",
		serviceAccountName:       "linkerd-service-mirror-remote-access-default",
		dockerRegistry:           "cr.l5d.io/linkerd",
		serviceMirrorRetryLimit:  defaults.ServiceMirrorRetryLimit,
		logLevel:                 defaults.LogLevel,
		selector:                 fmt.Sprintf("%s=%s", "mirror.linkerd.io/exported", "true"),
		remoteDiscoverySelector:  fmt.Sprintf("%s=%s", "mirror.linkerd.io/exported", "remote-discovery"),
		federatedServiceSelector: fmt.Sprintf("%s=%s", "mirror.linkerd.io/federated", "member"),
		gatewayAddresses:         "",
		gatewayPort:              0,
		ha:                       false,
		enableGateway:            true,
		output:                   "yaml",
		gatewayName:              "linkerd-gateway",
		gatewayNamespace:         "linkerd-multicluster",
	}, nil
}

/*
func NewLinkValues() (*Values, error) {
	// Use path relative to the templates root directory
	chartDir := "linkerd-multicluster-link/"
	v, err := readDefaults(chartDir)
	if err != nil {
		return nil, err
	}

	v.CliVersion = k8s.CreatedByAnnotationValue()
	return v, nil
}

func readDefaults(chartDir string) (*Values, error) {
	file := &loader.BufferedFile{
		Name: chartutil.ValuesfileName,
	}
	// Define the file system root
	var templates http.FileSystem = http.Dir("pkg/linkerd/charts")

	// For debugging, you could add:
	// fmt.Printf("Looking for values.yaml in: pkg/linkerd/charts/%s\n", chartDir)

	if err := charts.ReadFile(templates, chartDir, file); err != nil {
		return nil, fmt.Errorf("failed to read values.yaml: %w", err)
	}
	values := Values{}
	if err := yaml.Unmarshal(charts.InsertVersion(file.Data), &values); err != nil {
		return nil, err
	}
	return &values, nil
}
*/

func NewLinkValues() *Values {
	return &Values{
		ControllerImage:        "cr.l5d.io/linkerd/controller",
		ControllerImageVersion: "edge-24.2.5",
		EnablePodAntiAffinity:  true,
		Gateway: &Gateway{
			Enabled: true,
			Probe: &Probe{
				Port: 4191,
			},
		},
		LogLevel:                "info",
		LogFormat:               "plain",
		Replicas:                1,
		ServiceMirrorRetryLimit: 3,
		ServiceMirrorUID:        2103,
		ServiceMirrorGID:        2103,
		RevisionHistoryLimit:    10,
	}
}
