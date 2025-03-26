package clustershift

import (
	"clustershift/internal/logger"
	"clustershift/internal/prompt"
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
			logger.Info("Starting migration process...")
			logger.Info("You will be prompted to select a networking tool and rerouting option to establish a secure connection and manage traffic between the clusters.")

			opts := prompt.MigrationPrompt()
			migration.Migrate(kubeconfig1, kubeconfig2, opts)
			logger.Info("Migration complete")
		},
	}
)

func init() {
	migrateCluster.Flags().StringVarP(&kubeconfig1, "origin", "o", "", "Specify the path of the kubeconfig for the origin cluster")
	migrateCluster.Flags().StringVarP(&kubeconfig2, "target", "t", "", "Specify the path of the kubeconfig for the target cluster")
	rootCmd.AddCommand(migrateCluster)
}
