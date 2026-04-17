package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gojira",
	Short: "A CLI tool for interacting with Jira",
	Long: `gojira is a command-line interface tool for interacting with Jira.
It allows you to view your sprint board and log work to issues.

Configuration is loaded from a .env file in the current directory.`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Add subcommands here
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(meetingsCmd)
	rootCmd.AddCommand(summaryCmd)
	rootCmd.AddCommand(moveCmd)
	rootCmd.AddCommand(timerCmd)
	rootCmd.AddCommand(boardCmd)
}
