package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/orbitlink/orbitlink-machine/internal/config"
	"github.com/orbitlink/orbitlink-machine/internal/service"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the OrbitLink machine client",
	Long:  "Load local machine config and run until interrupted.",
	RunE:  runMachine,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runMachine(cmd *cobra.Command, _ []string) error {
	workDir, err := config.ResolveWorkDir(workDirFlag)
	if err != nil {
		return err
	}

	machineCfg, err := config.Load(workDir)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	svc, err := service.New(machineCfg)
	if err != nil {
		return err
	}

	if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
