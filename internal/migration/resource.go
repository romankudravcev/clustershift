package migration

import (
	"clustershift/internal/kube"
	"clustershift/internal/prompt"
	"clustershift/pkg/linkerd"
	"clustershift/pkg/submariner"
	"fmt"
)

type Resources interface {
	InstallNetworkingTool(clusters kube.Clusters)
	GetDNSName(name, namespace string) string
	ExportService(c kube.Cluster, namespace string, name string)
}

type SubmarinerResources struct{}

func (s *SubmarinerResources) InstallNetworkingTool(clusters kube.Clusters) {
	submariner.Install(clusters)
}

func (s *SubmarinerResources) GetDNSName(name, namespace string) string {
	return fmt.Sprintf("origin.%s-rw.%s.svc.clusterset.local", name, namespace)
}

func (s *SubmarinerResources) ExportService(c kube.Cluster, namespace string, name string) {
	submariner.Export(c, namespace, name, "")
}

type LinkerdResources struct{}

func (l *LinkerdResources) InstallNetworkingTool(clusters kube.Clusters) {
	linkerd.Install(clusters)
}

func (l *LinkerdResources) GetDNSName(name, namespace string) string {
	return fmt.Sprintf("%s-rw-origin.%s.svc.cluster.local", name, namespace)
}

func (l *LinkerdResources) ExportService(c kube.Cluster, namespace string, name string) {
	linkerd.ExportService(c, name, namespace)
}

func GetMigrationResources(tool string) (Resources, error) {
	switch tool {
	case prompt.NetworkingToolSubmariner:
		return &SubmarinerResources{}, nil
	case prompt.NetworkingToolLinkerd:
		return &LinkerdResources{}, nil
	default:
		return nil, fmt.Errorf("unsupported networking tool: %s", tool)
	}
}
