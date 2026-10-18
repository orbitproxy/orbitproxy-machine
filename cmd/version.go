package main

import (
	"fmt"
	"io"

	"github.com/orbitproxy/orbitproxy-machine/cliout"
	"github.com/spf13/cobra"
)

var versionJSONFlag bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the client version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return printVersion(cliout.Printer{JSON: versionJSONFlag, Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr()})
	},
}

func init() {
	versionCmd.Flags().BoolVar(&versionJSONFlag, "json", false, "print JSON")
	rootCmd.AddCommand(versionCmd)
}

func printVersion(printer cliout.Printer) error {
	version := clientVersion()
	return printer.Success(map[string]string{"version": version}, func(out io.Writer) {
		fmt.Fprintln(out, version)
	})
}
