package migration

import (
	"clustershift/internal/kube"
	"clustershift/internal/prompt"
	"clustershift/pkg/linkerd"
	"clustershift/pkg/skupper"
	"clustershift/pkg/submariner"
	"fmt"
)

type Resources interface {
	InstallNetworkingTool(clusters kube.Clusters)
	GetDNSName(name, namespace string) string
	GetPostgresDNSName(name, namespace string) string
	GetHeadlessDNSName(podName, serviceName, namespace, clusterId string) string
	ExportService(c kube.Cluster, namespace string, name string)
	GetNetworkingTool() string
	GetCNPGHostname(clusterName, dbClusterName, namespace string) string
}

type SubmarinerResources struct {
	networkingTool string
}

func (s *SubmarinerResources) InstallNetworkingTool(clusters kube.Clusters) {
	submariner.Install(clusters)
}

func (s *SubmarinerResources) GetDNSName(name, namespace string) string {
	return fmt.Sprintf("origin.%s-rw.%s.svc.clusterset.local", name, namespace)
}

func (s *SubmarinerResources) GetPostgresDNSName(name, namespace string) string {
	return fmt.Sprintf("origin.%s.%s.svc.clusterset.local", name, namespace)
}

func (s *SubmarinerResources) GetHeadlessDNSName(podName, serviceName, namespace, clusterId string) string {
	return fmt.Sprintf("%s.%s.%s.%s.svc.clusterset.local", podName, clusterId, serviceName, namespace)
}

func (s *SubmarinerResources) ExportService(c kube.Cluster, namespace string, name string) {
	submariner.Export(c, namespace, name, "")
}

func (s *SubmarinerResources) GetNetworkingTool() string {
	return s.networkingTool
}

func (s *SubmarinerResources) GetCNPGHostname(clusterName, dbClusterName, namespace string) string {
	return fmt.Sprintf("%s.%s-rw.%s.svc.clusterset.local", clusterName, dbClusterName, namespace)
}

type LinkerdResources struct {
	networkingTool string
}

func (l *LinkerdResources) InstallNetworkingTool(clusters kube.Clusters) {
	linkerd.Install(clusters)
}

func (l *LinkerdResources) GetDNSName(name, namespace string) string {
	return fmt.Sprintf("%s-rw-origin.%s.svc.cluster.local", name, namespace)
}

func (l *LinkerdResources) GetPostgresDNSName(name, namespace string) string {
	return fmt.Sprintf("%s-origin.%s.svc.cluster.local", name, namespace)
}

// GetHeadlessDNSName TODO - This is a temporary solution, we need to find a way to handle headless services properly
func (l *LinkerdResources) GetHeadlessDNSName(podName, serviceName, namespace, clusterId string) string {
	return fmt.Sprintf("%s.%s.%s.%s.svc.clusterset.local", podName, clusterId, serviceName, namespace)
}

func (l *LinkerdResources) ExportService(c kube.Cluster, namespace string, name string) {
	linkerd.ExportService(c, name, namespace)
}

func (l *LinkerdResources) GetNetworkingTool() string {
	return l.networkingTool
}

func (l *LinkerdResources) GetCNPGHostname(clusterName, dbClusterName, namespace string) string {
	// For Linkerd, the mirrored service name is constructed as: {original-service-name}-rw-{cluster-name}
	// This matches how addRWServiceToYaml constructs the service name
	return fmt.Sprintf("%s-rw-%s.%s.svc.cluster.local", dbClusterName, clusterName, namespace)
}

type SkupperResources struct {
	networkingTool string
}

func (s *SkupperResources) InstallNetworkingTool(clusters kube.Clusters) {
	skupper.Install(clusters)
}

func (s *SkupperResources) GetDNSName(name, namespace string) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", name, namespace)
}

func (s *SkupperResources) GetPostgresDNSName(name, namespace string) string {
	return fmt.Sprintf("%s-origin.%s.svc.cluster.local", name, namespace)
}

// GetHeadlessDNSName TODO - This is a temporary solution, we need to find a way to handle headless services properly
func (s *SkupperResources) GetHeadlessDNSName(podName, serviceName, namespace, clusterId string) string {
	return fmt.Sprintf("%s.%s.%s.%s.svc.clusterset.local", podName, clusterId, serviceName, namespace)
}

func (s *SkupperResources) ExportService(c kube.Cluster, namespace string, name string) {
	skupper.ExportService(c, namespace, name)
}

func (s *SkupperResources) GetNetworkingTool() string {
	return s.networkingTool
}

func (s *SkupperResources) GetCNPGHostname(clusterName, dbClusterName, namespace string) string {
	return fmt.Sprintf("%s-rw-%s.%s.svc.cluster.local", dbClusterName, clusterName, namespace)
}

func GetMigrationResources(tool string) (Resources, error) {
	switch tool {
	case prompt.NetworkingToolSubmariner:
		return &SubmarinerResources{networkingTool: tool}, nil
	case prompt.NetworkingToolLinkerd:
		return &LinkerdResources{networkingTool: tool}, nil
	case prompt.NetworkingToolSkupper:
		return &SkupperResources{networkingTool: tool}, nil
	default:
		return nil, fmt.Errorf("unsupported networking tool: %s", tool)
	}
}
