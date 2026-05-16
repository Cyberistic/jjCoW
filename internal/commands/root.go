package commands

import (
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "jjw",
	Short: "jj workspace manager",
	Long: `jjw is a CLI for managing jj workspaces with lifecycle hooks.

Create isolated environments for running multiple LLM coding agents
in parallel, each with their own workspace and bookmark.

Get started by creating a .jjw.yaml in your repository root:

  version: 1
  workspace_dir: workspaces
  default_branch: main`,
	SilenceUsage: true,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
