package commands

import (
	"fmt"

	"github.com/Cyberistic/jjCoW/internal/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(exitCmd)
}

var exitCmd = &cobra.Command{
	Use:   "exit",
	Short: "Return to the main repository",
	Long: `Output the path to the main repository root.

The shell integration wrapper will use this output to change
to the main repository directory.

Note: This command requires shell integration. Add this to your
shell rc file:

  eval "$(jjw init zsh)"  # or bash/fish`,
	RunE: runExit,
}

func runExit(cmd *cobra.Command, args []string) error {
	repoRoot, err := config.GetMainRepoRoot()
	if err != nil {
		return fmt.Errorf("not in a jjw-enabled repository: %w", err)
	}

	if !config.Exists(repoRoot) {
		return fmt.Errorf("not in a jjw-enabled repository (no .jjw.yaml found)")
	}

	fmt.Fprintln(cmd.OutOrStdout(), repoRoot)
	return nil
}
