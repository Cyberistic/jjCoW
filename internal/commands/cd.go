package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aranw/jjw/internal/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cdCmd)
}

var cdCmd = &cobra.Command{
	Use:   "cd <n>",
	Short: "Change to a workspace directory",
	Long: `Output the path to a workspace directory.

The shell integration wrapper will use this output to change
to the workspace directory.

Note: This command requires shell integration. Add this to your
shell rc file:

  eval "$(jjw init zsh)"  # or bash/fish`,
	Args: cobra.ExactArgs(1),
	RunE: runCd,
}

func runCd(cmd *cobra.Command, args []string) error {
	name := args[0]

	repoRoot, err := config.GetMainRepoRoot()
	if err != nil {
		return fmt.Errorf("not in a jjw-enabled repository: %w", err)
	}

	cfg, err := config.Load(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	workspacePath := filepath.Join(repoRoot, cfg.WorkspaceDir, name)

	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return fmt.Errorf("workspace %q does not exist", name)
	}

	fmt.Fprintln(cmd.OutOrStdout(), workspacePath)
	return nil
}
