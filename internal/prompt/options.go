package prompt

const (
	NetworkingToolSubmariner = "Submariner"
	NetworkingToolLinkerd    = "Linkerd"
	NetworkingToolSkupper    = "Skupper"

	ReroutingClustershift = "Clustershift"
	ReroutingSubmariner   = "Submariner"
	ReroutingLinkerd      = "Linkerd"
	ReroutingSkupper      = "Skupper"
)

type MigrationOptions struct {
	NetworkingTool string
	Rerouting      string
}
