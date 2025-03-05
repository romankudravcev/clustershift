package clustershift

import (
	"clustershift/internal/logger"
	"clustershift/internal/prompt"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	test = &cobra.Command{
		Use:   "test",
		Short: "test",
		Run: func(cmd *cobra.Command, args []string) {
			name := prompt.String("Enter your name:")

			options := []string{"Option 1", "Option 2", "Option 3"}

			selected := prompt.Select("Select an option:", options)

			logger.Info(fmt.Sprintf("Hello, %s! You chose: %s", name, selected))
			logger.Info("Migration started")
			logger.Debug("Test Debug Test")
		},
	}
)

func init() {
	rootCmd.AddCommand(test)
}
