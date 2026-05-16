package commands

import (
	"fmt"

	"github.com/aranw/jjw/internal/config"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(rootPathCmd)
}

var rootPathCmd = &cobra.Command{
	Use:   "root",
	Short: "Print the main repository root path",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := config.GetMainRepoRoot()
		if err != nil {
			return fmt.Errorf("not in a jjw-enabled repository: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), repoRoot)
		return nil
	},
}
