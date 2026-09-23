package main

import (
	"github.com/orbitproxy/orbitproxy-machine/buildmeta"
	"github.com/orbitproxy/orbitproxy-machine/httpclient"
	"github.com/spf13/cobra"
)

var workDirFlag string

var rootCmd = &cobra.Command{
	Use:           "orbitproxy",
	Short:         "OrbitProxy client",
	Long:          "OrbitProxy connects your host to an OrbitProxy edge and forwards traffic to local services.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Name() == "reset" || cmd.Name() == "delete" || commandHasParentNamed(cmd, "service") {
			return nil
		}
		if err := buildmeta.Validate(); err != nil {
			return err
		}
		httpclient.Init()
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&workDirFlag, "workdir", "", "OrbitProxy data root (default ~/.orbitproxy); config/logs live under <root>/<machineKey>/")
}

func Execute() error {
	return rootCmd.Execute()
}

func commandHasParentNamed(cmd *cobra.Command, name string) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Name() == name {
			return true
		}
	}
	return false
}
