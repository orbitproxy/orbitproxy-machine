package main

import (
	"fmt"
	"os"
)

func main() {
	if maybeRunAsService() {
		return
	}
	if tryHandleExplorerLaunch() {
		return
	}
	if err := Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
