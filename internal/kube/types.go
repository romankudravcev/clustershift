package kube

type ResourceType interface {
	IsResourceType()
}

type K8sResourceType string
type TraefikResourceType string

const (
	Deployment      K8sResourceType     = "Deployment"
	ConfigMap       K8sResourceType     = "ConfigMap"
	Ingress         K8sResourceType     = "Ingress"
	Secret          K8sResourceType     = "Secret"
	Namespace       K8sResourceType     = "Namespace"
	Service         K8sResourceType     = "Service"
	ServiceAccount  K8sResourceType     = "ServiceAccount"
	ClusterRole     K8sResourceType     = "ClusterRole"
	ClusterRoleBind K8sResourceType     = "ClusterRoleBinding"
	Middleware      TraefikResourceType = "Middleware"
	IngressRoute    TraefikResourceType = "IngressRoute"
	IngressRouteTCP TraefikResourceType = "IngressRouteTCP"
	IngressRouteUDP TraefikResourceType = "IngressRouteUDP"
	TraefikService  TraefikResourceType = "TraefikService"
)

func (K8sResourceType) IsResourceType()     {}
func (TraefikResourceType) IsResourceType() {}
