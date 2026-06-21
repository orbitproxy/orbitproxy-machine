package main

import (
	"os"

	"github.com/orbitlink/orbitlink-machine/internal/logger"
)

func main() {
	if err := Execute(); err != nil {
		logger.Error(logger.Multi).Err(err).Msg("orbitlink-machine command failed")
		os.Exit(1)
	}
}
