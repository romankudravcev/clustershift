package cnpg

import (
	"clustershift/internal/cli"
	"clustershift/internal/constants"
	"clustershift/internal/exit"
	"clustershift/internal/kube"
)

func InstallOperator(c kube.Cluster, logger *cli.Logger) {
	l := logger.Log("Installing cloudnative-pg operator")
	err := c.CreateResourcesFromURL(constants.CNPGOperatorURL)
	exit.OnErrorWithMessage(l.Fail("failed installing cloudnative-pg operator", err))
	l.Success("Installed cloudnative-pg operator")
}
