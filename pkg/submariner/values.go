package submariner

import (
	"bufio"
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
)

func GenerateJoinArgs(s SubmarinerJoinOptions) (string, error) {
	valuesTemplate := fmt.Sprintf(`
ipsec:
  psk: "%s"

broker:
  server: "%s"
  token: "%s"
  namespace: "%s"
  ca: "%s"
  globalnet: true

submariner:
  serviceDiscovery: true
  cableDriver: libreswan
  clusterId: "%s"
  clusterCidr: "%s"
  serviceCidr: "%s"
  globalCidr: "%s"
  natEnabled: true

serviceAccounts:
  globalnet:
    create: true
  lighthouseAgent:
    create: true
  lighthouseCoreDns:
    create: true
`,
		s.Psk,
		s.BrokerURL,
		s.Token,
		constants.SubmarinerBrokerNamespace,
		s.CA,
		s.ClusterId,
		s.PodCIDR,
		s.ServiceCIDR,
		s.GlobalCIDR,
	)

	return valuesTemplate, nil
}

func BuildCIDRs(c kube.Clusters) *CIDRs {
	podCIDROrigin := promptForInput("Enter Pod CIDR for origin cluster: ")
	podCIDRTarget := promptForInput("Enter Pod CIDR for target cluster: ")
	serviceCIDROrigin := promptForInput("Enter Service CIDR for origin cluster (blank for automatic detection): ")
	serviceCIDRTarget := promptForInput("Enter Service CIDR for target cluster (blank for automatic detection): ")
	brokerURL := promptForInput("Enter broker URL (blank for automatic detection): ")

	serviceCIDROrigin = fetchOrPrompt(serviceCIDROrigin, func() (string, error) { return c.Origin.FetchServiceCIDRs() }, "origin", "Service CIDR")
	serviceCIDRTarget = fetchOrPrompt(serviceCIDRTarget, func() (string, error) { return c.Target.FetchServiceCIDRs() }, "target", "Service CIDR")
	brokerURL = fetchOrPrompt(brokerURL, func() (string, error) { return c.Origin.FetchKubernetesAPIEndpoint() }, "", "Kubernetes API endpoint")

	if podCIDROrigin == "" || podCIDRTarget == "" {
		logger.Debug("Pod CIDRs are required for both clusters. Please provide them.")
		podCIDROrigin = promptForInput("Enter Pod CIDR for origin cluster: ")
		podCIDRTarget = promptForInput("Enter Pod CIDR for target cluster: ")
	}

	if podCIDROrigin == "" {
		exit.OnErrorWithMessage(fmt.Errorf("pod CIDR cannot be empty"), "Pod CIDR for origin cluster cannot be empty")
	}
	if podCIDRTarget == "" {
		exit.OnErrorWithMessage(fmt.Errorf("pod CIDR cannot be empty"), "Pod CIDR for target cluster cannot be empty")
	}

	logger.Debug(fmt.Sprintf("Pod CIDR Origin: %s\n", podCIDROrigin))
	logger.Debug(fmt.Sprintf("Pod CIDR Target: %s\n", podCIDRTarget))
	logger.Debug(fmt.Sprintf("Service CIDR Origin: %s\n", serviceCIDROrigin))
	logger.Debug(fmt.Sprintf("Service CIDR Target: %s\n", serviceCIDRTarget))
	logger.Debug(fmt.Sprintf("Broker URL: %s\n", brokerURL))

	return &CIDRs{
		podCIDROrigin:     podCIDROrigin,
		podCIDRTarget:     podCIDRTarget,
		serviceCIDROrigin: serviceCIDROrigin,
		serviceCIDRTarget: serviceCIDRTarget,
		brokerURL:         brokerURL,
	}
}

func fetchOrPrompt(value string, fetchFunc func() (string, error), clusterType, description string) string {
	if value == "" {
		fetchedValue, err := fetchFunc()
		if err != nil || fetchedValue == "" {
			value = promptForInput(fmt.Sprintf("Could not fetch %s for %s cluster. Please enter it manually:", description, clusterType))
		} else {
			value = fetchedValue
		}
	}
	return value
}

func promptForInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	input, _ := reader.ReadString('\n')
	return input[:len(input)-1] // Remove the newline character
}

func GenerateRandomString(length int) string {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		logger.Debug(fmt.Sprintf("Error generating random string"))
		return ""
	}
	logger.Debug(fmt.Sprintf("Generated random string: %s\n", base64.URLEncoding.EncodeToString(bytes)[:length]))
	return base64.URLEncoding.EncodeToString(bytes)[:length]
}
