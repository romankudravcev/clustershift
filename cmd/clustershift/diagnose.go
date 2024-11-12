package clustershift

import (
	"clustershift/pkg/connectivity"

	"github.com/spf13/cobra"
)

var (
	cluster1 string
	cluster2 string

	diagnose = &cobra.Command{
		Use:   "diagnose",
		Short: "diagnose connectivity between two clusters",
		Run: func(cmd *cobra.Command, args []string) {
			connectivity.DiagnoseConnection(cluster1, cluster2)
		},
	}
)

func init() {
	diagnose.Flags().StringVarP(&cluster1, "origin", "o", "", "Specify the path of the kubeconfig for the origin cluster")
	diagnose.Flags().StringVarP(&cluster2, "target", "t", "", "Specify the path of the kubeconfig for the target cluster")

	// Mark flags as required
	diagnose.MarkFlagRequired("origin")
	diagnose.MarkFlagRequired("target")

	rootCmd.AddCommand(diagnose)
}
