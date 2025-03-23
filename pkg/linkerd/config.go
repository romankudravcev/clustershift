// This code is copied from "https://github.com/linkerd/linkerd2" repository
// and has been modified for our specific use case.

package linkerd

type LinkerdConfig struct {
	ClusterDomain                string                `yaml:"clusterDomain"`
	ClusterNetworks              string                `yaml:"clusterNetworks"`
	CniEnabled                   bool                  `yaml:"cniEnabled"`
	CommonLabels                 map[string]string     `yaml:"commonLabels"`
	ControlPlaneTracing          bool                  `yaml:"controlPlaneTracing"`
	ControlPlaneTracingNamespace string                `yaml:"controlPlaneTracingNamespace"`
	Controller                   Controller            `yaml:"controller"`
	ControllerGID                int                   `yaml:"controllerGID"`
	ControllerImage              string                `yaml:"controllerImage"`
	ControllerImageVersion       string                `yaml:"controllerImageVersion"`
	ControllerLogFormat          string                `yaml:"controllerLogFormat"`
	ControllerLogLevel           string                `yaml:"controllerLogLevel"`
	ControllerReplicas           int                   `yaml:"controllerReplicas"`
	ControllerUID                int                   `yaml:"controllerUID"`
	DebugContainer               DebugContainer        `yaml:"debugContainer"`
	DeploymentStrategy           DeploymentStrategy    `yaml:"deploymentStrategy"`
	DestinationController        DestinationController `yaml:"destinationController"`
	DisableHeartBeat             bool                  `yaml:"disableHeartBeat"`
	DisableIPv6                  bool                  `yaml:"disableIPv6"`
	Egress                       Egress                `yaml:"egress"`
	EnableEndpointSlices         bool                  `yaml:"enableEndpointSlices"`
	EnableH2Upgrade              bool                  `yaml:"enableH2Upgrade"`
	EnablePSP                    bool                  `yaml:"enablePSP"`
	EnablePodAntiAffinity        bool                  `yaml:"enablePodAntiAffinity"`
	EnablePodDisruptionBudget    bool                  `yaml:"enablePodDisruptionBudget"`
	EnablePprof                  bool                  `yaml:"enablePprof"`
	Identity                     Identity              `yaml:"identity"`
	IdentityTrustAnchorsPEM      string                `yaml:"identityTrustAnchorsPEM"`
	IdentityTrustDomain          string                `yaml:"identityTrustDomain"`
	ImagePullPolicy              string                `yaml:"imagePullPolicy"`
	ImagePullSecrets             []string              `yaml:"imagePullSecrets"`
	KubeAPI                      KubeAPI               `yaml:"kubeAPI"`
	LinkerdVersion               string                `yaml:"linkerdVersion"`
	NetworkValidator             NetworkValidator      `yaml:"networkValidator"`
	NodeSelector                 map[string]string     `yaml:"nodeSelector"`
	PodAnnotations               map[string]string     `yaml:"podAnnotations"`
	PodLabels                    map[string]string     `yaml:"podLabels"`
	PodMonitor                   PodMonitor            `yaml:"podMonitor"`
	PolicyController             PolicyController      `yaml:"policyController"`
	PolicyValidator              Validator             `yaml:"policyValidator"`
	PriorityClassName            string                `yaml:"priorityClassName"`
	ProfileValidator             Validator             `yaml:"profileValidator"`
	PrometheusUrl                string                `yaml:"prometheusUrl"`
	Proxy                        Proxy                 `yaml:"proxy"`
	ProxyInit                    ProxyInit             `yaml:"proxyInit"`
	ProxyInjector                ProxyInjector         `yaml:"proxyInjector"`
	RevisionHistoryLimit         int                   `yaml:"revisionHistoryLimit"`
	RuntimeClassName             string                `yaml:"runtimeClassName"`
	SpValidator                  Validator             `yaml:"spValidator"`
	WebhookFailurePolicy         string                `yaml:"webhookFailurePolicy"`
}

type Controller struct {
	PodDisruptionBudget struct {
		MaxUnavailable int `yaml:"maxUnavailable"`
	} `yaml:"podDisruptionBudget"`
}

type DebugContainer struct {
	Image struct {
		Name       string `yaml:"name"`
		PullPolicy string `yaml:"pullPolicy"`
		Version    string `yaml:"version"`
	} `yaml:"image"`
}

type DeploymentStrategy struct {
	RollingUpdate struct {
		MaxSurge       string `yaml:"maxSurge"`
		MaxUnavailable string `yaml:"maxUnavailable"`
	} `yaml:"rollingUpdate"`
}

type DestinationController struct {
	LivenessProbe struct {
		TimeoutSeconds int `yaml:"timeoutSeconds"`
	} `yaml:"livenessProbe"`
	MeshedHttp2ClientProtobuf struct {
		KeepAlive struct {
			Interval struct {
				Seconds int `yaml:"seconds"`
			} `yaml:"interval"`
			Timeout struct {
				Seconds int `yaml:"seconds"`
			} `yaml:"timeout"`
			WhileIdle bool `yaml:"while_idle"`
		} `yaml:"keep_alive"`
	} `yaml:"meshedHttp2ClientProtobuf"`
	PodAnnotations map[string]string `yaml:"podAnnotations"`
	ReadinessProbe struct {
		TimeoutSeconds int `yaml:"timeoutSeconds"`
	} `yaml:"readinessProbe"`
}

type Egress struct {
	GlobalEgressNetworkNamespace string `yaml:"globalEgressNetworkNamespace"`
}

type Identity struct {
	ExternalCA bool `yaml:"externalCA"`
	Issuer     struct {
		ClockSkewAllowance string `yaml:"clockSkewAllowance"`
		IssuanceLifetime   string `yaml:"issuanceLifetime"`
		Scheme             string `yaml:"scheme"`
		TLS                struct {
			CrtPEM string `yaml:"crtPEM"`
		} `yaml:"tls"`
	} `yaml:"issuer"`
	KubeAPI struct {
		ClientBurst int `yaml:"clientBurst"`
		ClientQPS   int `yaml:"clientQPS"`
	} `yaml:"kubeAPI"`
	LivenessProbe struct {
		TimeoutSeconds int `yaml:"timeoutSeconds"`
	} `yaml:"livenessProbe"`
	PodAnnotations map[string]string `yaml:"podAnnotations"`
	ReadinessProbe struct {
		TimeoutSeconds int `yaml:"timeoutSeconds"`
	} `yaml:"readinessProbe"`
	ServiceAccountTokenProjection bool `yaml:"serviceAccountTokenProjection"`
}

type KubeAPI struct {
	ClientBurst int `yaml:"clientBurst"`
	ClientQPS   int `yaml:"clientQPS"`
}

type NetworkValidator struct {
	ConnectAddr           string `yaml:"connectAddr"`
	EnableSecurityContext bool   `yaml:"enableSecurityContext"`
	ListenAddr            string `yaml:"listenAddr"`
	LogFormat             string `yaml:"logFormat"`
	LogLevel              string `yaml:"logLevel"`
	Timeout               string `yaml:"timeout"`
}

type PodMonitor struct {
	Controller struct {
		Enabled           bool   `yaml:"enabled"`
		NamespaceSelector string `yaml:"namespaceSelector"`
	} `yaml:"controller"`
	Enabled bool              `yaml:"enabled"`
	Labels  map[string]string `yaml:"labels"`
	Proxy   struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"proxy"`
	ScrapeInterval string `yaml:"scrapeInterval"`
	ScrapeTimeout  string `yaml:"scrapeTimeout"`
	ServiceMirror  struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"serviceMirror"`
}

type PolicyController struct {
	Image struct {
		Name       string `yaml:"name"`
		PullPolicy string `yaml:"pullPolicy"`
		Version    string `yaml:"version"`
	} `yaml:"image"`
	LivenessProbe struct {
		TimeoutSeconds int `yaml:"timeoutSeconds"`
	} `yaml:"livenessProbe"`
	LogLevel       string   `yaml:"logLevel"`
	ProbeNetworks  []string `yaml:"probeNetworks"`
	ReadinessProbe struct {
		TimeoutSeconds int `yaml:"timeoutSeconds"`
	} `yaml:"readinessProbe"`
	Resources struct {
		CPU struct {
			Limit   string `yaml:"limit"`
			Request string `yaml:"request"`
		} `yaml:"cpu"`
		EphemeralStorage struct {
			Limit   string `yaml:"limit"`
			Request string `yaml:"request"`
		} `yaml:"ephemeral-storage"`
		Memory struct {
			Limit   string `yaml:"limit"`
			Request string `yaml:"request"`
		} `yaml:"memory"`
	} `yaml:"resources"`
}

type Validator struct {
	LivenessProbe struct {
		TimeoutSeconds int `yaml:"timeoutSeconds"`
	} `yaml:"livenessProbe"`
	ReadinessProbe struct {
		TimeoutSeconds int `yaml:"timeoutSeconds"`
	} `yaml:"readinessProbe"`
	WebhookFailurePolicy string `yaml:"webhookFailurePolicy"`
}

type Proxy struct {
	Await                                bool         `yaml:"await"`
	Control                              Control      `yaml:"control"`
	Cores                                int          `yaml:"cores"`
	DefaultInboundPolicy                 string       `yaml:"defaultInboundPolicy"`
	DisableInboundProtocolDetectTimeout  bool         `yaml:"disableInboundProtocolDetectTimeout"`
	DisableOutboundProtocolDetectTimeout bool         `yaml:"disableOutboundProtocolDetectTimeout"`
	EnableExternalProfiles               bool         `yaml:"enableExternalProfiles"`
	EnableShutdownEndpoint               bool         `yaml:"enableShutdownEndpoint"`
	GID                                  int          `yaml:"gid"`
	Image                                ProxyImage   `yaml:"image"`
	Inbound                              Inbound      `yaml:"inbound"`
	InboundConnectTimeout                string       `yaml:"inboundConnectTimeout"`
	InboundDiscoveryCacheUnusedTimeout   string       `yaml:"inboundDiscoveryCacheUnusedTimeout"`
	LivenessProbe                        RProbe       `yaml:"livenessProbe"`
	LogFormat                            string       `yaml:"logFormat"`
	LogHTTPHeaders                       string       `yaml:"logHTTPHeaders"`
	LogLevel                             string       `yaml:"logLevel"`
	NativeSidecar                        bool         `yaml:"nativeSidecar"`
	OpaquePorts                          string       `yaml:"opaquePorts"`
	Outbound                             Outbound     `yaml:"outbound"`
	OutboundConnectTimeout               string       `yaml:"outboundConnectTimeout"`
	OutboundDiscoveryCacheUnusedTimeout  string       `yaml:"outboundDiscoveryCacheUnusedTimeout"`
	OutboundTransportMode                string       `yaml:"outboundTransportMode"`
	Ports                                Ports        `yaml:"ports"`
	ReadinessProbe                       RProbe       `yaml:"readinessProbe"`
	RequireIdentityOnInboundPorts        string       `yaml:"requireIdentityOnInboundPorts"`
	Resources                            Resources    `yaml:"resources"`
	ShutdownGracePeriod                  string       `yaml:"shutdownGracePeriod"`
	StartupProbe                         StartupProbe `yaml:"startupProbe"`
	UID                                  int          `yaml:"uid"`
	WaitBeforeExitSeconds                int          `yaml:"waitBeforeExitSeconds"`
}

type Control struct {
	Streams struct {
		IdleTimeout    string `yaml:"idleTimeout"`
		InitialTimeout string `yaml:"initialTimeout"`
		Lifetime       string `yaml:"lifetime"`
	} `yaml:"streams"`
}

type ProxyImage struct {
	Name       string `yaml:"name"`
	PullPolicy string `yaml:"pullPolicy"`
	Version    string `yaml:"version"`
}

type Inbound struct {
	Server struct {
		HTTP2 struct {
			KeepAliveInterval string `yaml:"keepAliveInterval"`
			KeepAliveTimeout  string `yaml:"keepAliveTimeout"`
		} `yaml:"http2"`
	} `yaml:"server"`
}

type RProbe struct {
	InitialDelaySeconds int `yaml:"initialDelaySeconds"`
	TimeoutSeconds      int `yaml:"timeoutSeconds"`
}

type Outbound struct {
	Server struct {
		HTTP2 struct {
			KeepAliveInterval string `yaml:"keepAliveInterval"`
			KeepAliveTimeout  string `yaml:"keepAliveTimeout"`
		} `yaml:"http2"`
	} `yaml:"server"`
}

type Ports struct {
	Admin    int `yaml:"admin"`
	Control  int `yaml:"control"`
	Inbound  int `yaml:"inbound"`
	Outbound int `yaml:"outbound"`
}

type Resources struct {
	CPU struct {
		Limit   string `yaml:"limit"`
		Request string `yaml:"request"`
	} `yaml:"cpu"`
	EphemeralStorage struct {
		Limit   string `yaml:"limit"`
		Request string `yaml:"request"`
	} `yaml:"ephemeral-storage"`
	Memory struct {
		Limit   string `yaml:"limit"`
		Request string `yaml:"request"`
	} `yaml:"memory"`
}

type StartupProbe struct {
	FailureThreshold    int `yaml:"failureThreshold"`
	InitialDelaySeconds int `yaml:"initialDelaySeconds"`
	PeriodSeconds       int `yaml:"periodSeconds"`
}

type ProxyInit struct {
	CloseWaitTimeoutSecs int    `yaml:"closeWaitTimeoutSecs"`
	IgnoreInboundPorts   string `yaml:"ignoreInboundPorts"`
	IgnoreOutboundPorts  string `yaml:"ignoreOutboundPorts"`
	Image                struct {
		Name       string `yaml:"name"`
		PullPolicy string `yaml:"pullPolicy"`
		Version    string `yaml:"version"`
	} `yaml:"image"`
	IptablesMode       string `yaml:"iptablesMode"`
	KubeAPIServerPorts string `yaml:"kubeAPIServerPorts"`
	LogFormat          string `yaml:"logFormat"`
	LogLevel           string `yaml:"logLevel"`
	Privileged         bool   `yaml:"privileged"`
	RunAsGroup         int    `yaml:"runAsGroup"`
	RunAsRoot          bool   `yaml:"runAsRoot"`
	RunAsUser          int    `yaml:"runAsUser"`
	SkipSubnets        string `yaml:"skipSubnets"`
	XtMountPath        struct {
		MountPath string `yaml:"mountPath"`
		Name      string `yaml:"name"`
	} `yaml:"xtMountPath"`
}

type ProxyInjector struct {
	CaBundle           string `yaml:"caBundle"`
	CrtPEM             string `yaml:"crtPEM"`
	ExternalSecret     bool   `yaml:"externalSecret"`
	InjectCaFrom       string `yaml:"injectCaFrom"`
	InjectCaFromSecret string `yaml:"injectCaFromSecret"`
	LivenessProbe      struct {
		TimeoutSeconds int `yaml:"timeoutSeconds"`
	} `yaml:"livenessProbe"`
	NamespaceSelector struct {
		MatchExpressions []struct {
			Key      string   `yaml:"key"`
			Operator string   `yaml:"operator"`
			Values   []string `yaml:"values"`
		} `yaml:"matchExpressions"`
	} `yaml:"namespaceSelector"`
	ObjectSelector struct {
		MatchExpressions []struct {
			Key      string `yaml:"key"`
			Operator string `yaml:"operator"`
		} `yaml:"matchExpressions"`
	} `yaml:"objectSelector"`
	PodAnnotations map[string]string `yaml:"podAnnotations"`
	ReadinessProbe struct {
		TimeoutSeconds int `yaml:"timeoutSeconds"`
	} `yaml:"readinessProbe"`
	TimeoutSeconds int `yaml:"timeoutSeconds"`
}
