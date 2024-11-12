package exit

import (
	"fmt"
	"os"
)

func OnError(err error) {
	if err != nil {
		os.Exit(1)
	}
}

func OnErrorWithMessage(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "")
		os.Exit(1)
	}
}
