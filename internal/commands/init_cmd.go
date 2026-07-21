package commands

import (
	"fmt"

	"github.com/Cyberistic/jjCoW/internal/shell"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init <shell>",
	Short: "Generate shell integration script",
	Long: `Generate a shell integration script for the given shell.

Supported shells: zsh, bash, fish

Add to your shell config:

  # zsh (~/.zshrc)
  eval "$(jjw init zsh)"

  # bash (~/.bashrc)
  eval "$(jjw init bash)"

  # fish (~/.config/fish/config.fish)
  jjw init fish | source`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"zsh", "bash", "fish"},
	RunE: func(cmd *cobra.Command, args []string) error {
		script, err := shell.Generate(args[0])
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), script)
		return nil
	},
}
