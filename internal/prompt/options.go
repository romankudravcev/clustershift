package prompt

const (
	NetworkingToolSubmariner = "Submariner"
	NetworkingToolLinkerd    = "Linkerd"

	ReroutingClustershift = "Clustershift"
	ReroutingSubmariner   = "Submariner"
	ReroutingLinkerd      = "Linkerd"
)

type MigrationOptions struct {
	NetworkingTool string
	Rerouting      string
}
