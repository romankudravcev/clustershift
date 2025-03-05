package exit

import (
	"clustershift/internal/logger"
	"os"
)

func OnError(err error) {
	if err != nil {
		os.Exit(1)
	}
}

func OnErrorWithMessage(err error, message string) {
	if err != nil {
		logger.Error(message + ": " + err.Error())
		os.Exit(1)
	}
}
