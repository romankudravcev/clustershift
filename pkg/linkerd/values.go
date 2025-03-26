// This code is copied from "https://github.com/linkerd/linkerd2" repository
// and has been modified for our specific use case.

package linkerd

import (
	"fmt"
	multicluster "github.com/linkerd/linkerd2/multicluster/values"
	"github.com/linkerd/linkerd2/pkg/version"
)

const (
	clusterNameLabel        = "multicluster.linkerd.io/cluster-name"
	trustDomainAnnotation   = "multicluster.linkerd.io/trust-domain"
	clusterDomainAnnotation = "multicluster.linkerd.io/cluster-domain"
)

type (
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
		enableServiceMirror      bool
		output                   string
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
		logFormat:                defaults.LogFormat,
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
func NewLinkValues() *Values {
	return &mValues{
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
*/
