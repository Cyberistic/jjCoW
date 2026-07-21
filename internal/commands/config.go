package commands

import (
	"fmt"
	"os"

	"github.com/Cyberistic/jjCoW/internal/config"

	"github.com/spf13/cobra"
)

const defaultConfig = `# jjw configuration
# See: https://github.com/Cyberistic/jjCoW

version: 1

# Directory where workspaces are created (relative to this file)
workspace_dir: workspaces

# Bookmark naming pattern ({name} is replaced with the workspace name)
bookmark_pattern: "{name}"

# Branch to compare against for status (ahead/behind/merged)
default_branch: main

# Subdirectory containing the jj repository (use "." if .jjw.yaml is inside the repo)
repo_dir: "."

# Copy-on-write workspace cloning (APFS on macOS, reflinks on Linux).
# Clones the working copy instead of waiting for a full jj checkout, and
# carries over untracked files like .env. Disable to always do a full checkout.
cow: true

# Defer jj adoption of cloned files (fastest creation). When true, you must
# run "jj sparse reset" in the new workspace before using jj commands there.
cow_lazy: false
`

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configGetCmd)

	configInitCmd.Flags().BoolP("force", "f", false, "Overwrite existing .jjw.yaml")
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage jjw configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Print a configuration value",
	Long: `Print a configuration value from .jjw.yaml.

Supported keys:
  workspace_dir
  bookmark_pattern
  default_branch
  repo_dir
  track_remote
  cow
  cow_lazy`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := config.GetMainRepoRoot()
		if err != nil {
			return fmt.Errorf("not in a jjw-enabled repository: %w", err)
		}

		cfg, err := config.Load(repoRoot)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		switch args[0] {
		case "workspace_dir":
			fmt.Fprintln(cmd.OutOrStdout(), cfg.WorkspaceDir)
		case "bookmark_pattern":
			fmt.Fprintln(cmd.OutOrStdout(), cfg.BookmarkPattern)
		case "default_branch":
			fmt.Fprintln(cmd.OutOrStdout(), cfg.DefaultBranch)
		case "repo_dir":
			fmt.Fprintln(cmd.OutOrStdout(), cfg.RepoDir)
		case "track_remote":
			fmt.Fprintln(cmd.OutOrStdout(), cfg.TrackRemote)
		case "cow":
			fmt.Fprintln(cmd.OutOrStdout(), cfg.Cow)
		case "cow_lazy":
			fmt.Fprintln(cmd.OutOrStdout(), cfg.CowLazy)
		default:
			return fmt.Errorf("unknown config key %q", args[0])
		}
		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default .jjw.yaml configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")

		const filename = ".jjw.yaml"

		if !force {
			if _, err := os.Stat(filename); err == nil {
				return fmt.Errorf("%s already exists (use --force to overwrite)", filename)
			}
		}

		if err := os.WriteFile(filename, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", filename)
		return nil
	},
}
