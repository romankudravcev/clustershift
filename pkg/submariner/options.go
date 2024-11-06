package submariner

type SubmarinerJoinOptions struct {
	Psk         string
	BrokerURL   string
	Token       string
	CA          string
	ClusterId   string
	PodCIDR     string
	ServiceCIDR string
	GlobalCIDR  string
}

type CIDRs struct {
	podCIDROrigin     string
	podCIDRTarget     string
	serviceCIDROrigin string
	serviceCIDRTarget string
	brokerURL         string
}
