package main

import (
	"github.com/orbitlink/orbitlink-machine/internal/buildmeta"
	"github.com/orbitlink/orbitlink-machine/internal/httpclient"
	"github.com/spf13/cobra"
)

var workDirFlag string

var rootCmd = &cobra.Command{
	Use:           "orbitlink-machine",
	Short:         "OrbitLink machine client",
	Long:          "OrbitLink machine connects your host to an OrbitLink edge and forwards traffic to local services.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := buildmeta.Validate(); err != nil {
			return err
		}
		httpclient.Init()
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&workDirFlag, "workdir", "", "OrbitLink work directory (config and logs)")
}

func Execute() error {
	return rootCmd.Execute()
}
