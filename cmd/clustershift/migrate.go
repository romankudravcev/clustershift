package clustershift

import (
	"clustershift/pkg/migration"

	"github.com/spf13/cobra"
)

var (
	kubeconfig1 string
	kubeconfig2 string

	migrateCluster = &cobra.Command{
		Use:   "migrate",
		Short: "migrate origin cluster to target cluster",
		Run: func(cmd *cobra.Command, args []string) {
			migration.Migrate(kubeconfig1, kubeconfig2)
		},
	}
)

func init() {
	migrateCluster.Flags().StringVarP(&kubeconfig1, "origin", "o", "", "Specify the path of the kubeconfig for the origin cluster")
	migrateCluster.Flags().StringVarP(&kubeconfig2, "target", "t", "", "Specify the path of the kubeconfig for the target cluster")
	rootCmd.AddCommand(migrateCluster)
}
