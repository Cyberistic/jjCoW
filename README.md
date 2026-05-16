# jjw - jj workspace manager

A CLI for managing [jj](https://github.com/jj-vcs/jj) workspaces with lifecycle hooks. Create isolated, fully testable environments for running multiple LLM coding agents in parallel.

Inspired by [wt](https://github.com/agarcher/wt) (git worktree manager), adapted for jj's workspace and bookmark model.

## Installation

### From source

```
git clone git@github.com:aranw/jjw.git
cd jjw
just build
just install
```

Then add shell integration to your shell config:

```sh
# zsh (~/.zshrc)
eval "$(jjw init zsh)"

# bash (~/.bashrc)
eval "$(jjw init bash)"

# fish (~/.config/fish/config.fish)
jjw init fish | source
```

Restart your shell or source the config file.

**Requirements:** jj 0.25+ and a colocated jj/git repository.

## Quick start

1. Create a `.jjw.yaml` in your repository root:

```sh
jjw config init
```

Or create one manually:

```yaml
version: 1
workspace_dir: workspaces
default_branch: main
```

2. Create and use workspaces:

```sh
jjw create feature-x      # create workspace + bookmark, cd into it
jjw list                   # list all workspaces
jjw cd feature-x           # switch to a workspace
jjw exit                   # return to main repo
jjw delete feature-x       # delete workspace, bookmark, and files
jjw cleanup                # remove workspaces with merged bookmarks
```

## What happens when you run `jjw create`

When you run `jjw create feature-x`, the tool:

1. Runs `jj workspace add workspaces/feature-x --name feature-x -r main`
2. Creates a bookmark `feature-x` pointing at the new workspace's working copy (`@`)
3. Allocates a unique index for hook environment variables
4. Runs any `post_create` hooks
5. Changes directory into the new workspace (via shell integration)

You then work in the workspace, push the bookmark with `jj git push`, and open a PR as normal. Once merged, `jjw cleanup` will find it and tidy up.

## Commands

| Command | Description |
|---|---|
| `jjw create <n>` | Create a new workspace with a bookmark |
| `jjw delete [name]` | Delete a workspace, its files, and bookmark |
| `jjw list` | List all workspaces with status |
| `jjw cd <n>` | Change to a workspace directory |
| `jjw exit` | Return to main repository |
| `jjw cleanup` | Remove clean workspaces with merged bookmarks |
| `jjw init <shell>` | Generate shell integration |
| `jjw config init` | Create a default `.jjw.yaml` file |
| `jjw config get <key>` | Print a configuration value |
| `jjw root` | Print main repository path |
| `jjw version` | Print version |

### `jjw create`

```
jjw create <name> [-r revision] [-b bookmark]
```

- `-r, --revision` — base revision for the workspace (defaults to `default_branch` from config)
- `-b, --bookmark` — use an existing bookmark instead of creating a new one

### `jjw delete`

```
jjw delete [name] [-f] [-k]
```

- `-f, --force` — delete without warning about uncommitted or unmerged work
- `-k, --keep-bookmark` — keep the associated bookmark

If no name is provided and you're inside a workspace, that workspace is deleted.

Without `--force`, `jjw delete` warns and asks for confirmation if the workspace has a non-empty working copy or its work does not appear to be merged into `default_branch`. With `--force`, these safety warnings are skipped.

### `jjw cleanup`

```
jjw cleanup [-n] [-f] [-k]
```

- `-n, --dry-run` — show what would be deleted without deleting
- `-f, --force` — skip confirmation prompts and pre-delete hook failures
- `-k, --keep-bookmark` — keep the associated bookmarks

`jjw cleanup` only deletes workspaces whose work appears merged and whose working copy is empty. If an otherwise eligible workspace has uncommitted changes, cleanup cancels instead of deleting it.

## Configuration

### Repository configuration

Each repository is configured via `.jjw.yaml` at the repository root:

```yaml
version: 1
workspace_dir: workspaces          # where workspaces are stored
bookmark_pattern: "{name}"         # bookmark naming pattern
default_branch: main               # branch for comparison
repo_dir: "."                       # subdirectory containing the jj repo
track_remote: origin               # optional: auto-track bookmarks on this remote

index:
  max: 10                          # maximum workspace index (0 = no limit)

hooks:
  post_create:
    - script: ./scripts/setup.sh
  pre_delete:
    - script: ./scripts/teardown.sh
```

### Bookmark patterns

The `bookmark_pattern` field controls how bookmark names are derived from workspace names. Use `{name}` as a placeholder:

```yaml
bookmark_pattern: "{name}"              # feature-x → feature-x
bookmark_pattern: "ws/{name}"           # feature-x → ws/feature-x
bookmark_pattern: "aran/{name}"         # feature-x → aran/feature-x
```

## Hooks

Hooks run custom scripts at key points in the workspace lifecycle:

| Hook | When it runs |
|---|---|
| `pre_create` | Before workspace creation |
| `post_create` | After workspace creation |
| `pre_delete` | Before workspace deletion |
| `post_delete` | After workspace deletion |
| `info` | Reserved for future detailed workspace information |

All hooks receive environment variables:

| Variable | Description |
|---|---|
| `JJW_NAME` | Workspace name |
| `JJW_PATH` | Absolute path to workspace |
| `JJW_BOOKMARK` | Associated bookmark name |
| `JJW_REPO_ROOT` | Main repository root, where `.jjw.yaml` lives |
| `JJW_JJ_ROOT` | jj repository root, usually the same as `JJW_REPO_ROOT` unless `repo_dir` is set |
| `JJW_WORKSPACE_DIR` | Workspace directory name |
| `JJW_INDEX` | Unique workspace index (if allocated) |

### Example: unique dev server ports

```yaml
hooks:
  post_create:
    - script: ./scripts/setup-ports.sh
```

```bash
#!/bin/bash
# scripts/setup-ports.sh
PORT=$((3000 + JJW_INDEX))
echo "Dev server port: $PORT"
# Write port config, update .env, etc.
```

## Metadata

`jjw` stores per-workspace metadata in `.jjw/workspaces/<name>/` at the repository root. This includes creation timestamps and allocated indexes. Add `.jjw/` to your `.gitignore`.

## Differences from wt

`jjw` follows the same architecture and patterns as `wt`, adapted for jj:

| Concept | wt (git) | jjw (jj) |
|---|---|---|
| Isolation mechanism | git worktrees | jj workspaces |
| Branch/ref | git branch | jj bookmark |
| Config file | `.wt.yaml` | `.jjw.yaml` |
| Detection | `.git` file vs directory | `.jjw.yaml` walk-up |
| Env prefix | `WT_` | `JJW_` |
| Shell wrapper env | `WT_CD_FILE` | `JJW_CD_FILE` |

## Licence

MIT
