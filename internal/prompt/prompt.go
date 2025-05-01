package prompt

import (
	"clustershift/internal/exit"
	"github.com/AlecAivazis/survey/v2"
)

func String(message string) string {
	var result string
	prompt := &survey.Input{
		Message: message,
	}
	exit.OnErrorWithMessage(survey.AskOne(prompt, &result), "Failed to prompt for input")
	return result
}

func Select(message string, options []string) string {
	var selected string
	selectPrompt := &survey.Select{
		Message: message,
		Options: options,
	}
	exit.OnErrorWithMessage(survey.AskOne(selectPrompt, &selected), "Failed to prompt for select")

	return selected
}

func MigrationPrompt() MigrationOptions {
	networkingTool := Select("Select a networking tool", []string{NetworkingToolSubmariner + " (recommended)", NetworkingToolLinkerd, NetworkingToolSkupper})
	rerouting := Select("Select a rerouting option", []string{ReroutingClustershift + " (recommended)", ReroutingSubmariner, ReroutingLinkerd, ReroutingSkupper})

	return MigrationOptions{
		NetworkingTool: networkingTool,
		Rerouting:      rerouting,
	}
}
