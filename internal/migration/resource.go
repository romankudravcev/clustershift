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
	ExportService(c kube.Cluster, namespace string, name string)
	GetNetworkingTool() string // Added method to access the tool name
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

func (s *SubmarinerResources) ExportService(c kube.Cluster, namespace string, name string) {
	submariner.Export(c, namespace, name, "")
}

func (s *SubmarinerResources) GetNetworkingTool() string {
	return s.networkingTool
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

func (l *LinkerdResources) ExportService(c kube.Cluster, namespace string, name string) {
	linkerd.ExportService(c, name, namespace)
}

func (l *LinkerdResources) GetNetworkingTool() string {
	return l.networkingTool
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

func (s *SkupperResources) ExportService(c kube.Cluster, namespace string, name string) {
	skupper.ExportService(c, namespace, name)
}

func (s *SkupperResources) GetNetworkingTool() string {
	return s.networkingTool
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
