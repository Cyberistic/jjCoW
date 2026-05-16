package shell

import "fmt"

// GenerateZsh generates the zsh shell integration script.
func GenerateZsh() string {
	return `# jjw shell integration for zsh
# Add this to your ~/.zshrc: eval "$(jjw init zsh)"

_jjw() {
  local curcontext="$curcontext" state line
  typeset -A opt_args

  _arguments -C \
    '1: :->command' \
    '*: :->args'

  case $state in
    command)
      local commands=(
        'create:Create a new workspace'
        'delete:Delete a workspace'
        'cd:Change to a workspace directory'
        'list:List all workspaces'
        'cleanup:Clean up merged workspaces'
        'exit:Return to the main repository'
        'init:Generate shell integration script'
        'config:Manage jjw configuration'
        'root:Print the main repository root path'
        'version:Print the version number'
        'help:Help about any command'
      )
      _describe 'command' commands
      ;;
    args)
      case $words[2] in
        cd)
          local has_name=false
          for ((i=3; i < $CURRENT; i++)); do
            if [[ ${words[$i]} != -* ]]; then
              has_name=true
              break
            fi
          done
          if ! $has_name; then
            local repo_root workspace_dir workspaces
            repo_root=$(_jjw_find_root)
            if [[ -n "$repo_root" ]]; then
              workspace_dir=$(_jjw_workspace_dir "$repo_root")
              if [[ -d "$repo_root/$workspace_dir" ]]; then
                workspaces=(${(f)"$(ls -1 "$repo_root/$workspace_dir" 2>/dev/null)"})
                _describe 'workspace' workspaces
              fi
            fi
          fi
          ;;
        delete)
          local has_name=false
          for ((i=3; i < $CURRENT; i++)); do
            if [[ ${words[$i]} != -* ]]; then
              has_name=true
              break
            fi
          done
          if $has_name || [[ ${words[$CURRENT]} == -* ]]; then
            local -a flags=(
              '-f:Force deletion'
              '--force:Force deletion'
              '-k:Keep the associated bookmark'
              '--keep-bookmark:Keep the associated bookmark'
            )
            _describe 'flag' flags
          else
            local repo_root workspace_dir workspaces
            repo_root=$(_jjw_find_root)
            if [[ -n "$repo_root" ]]; then
              workspace_dir=$(_jjw_workspace_dir "$repo_root")
              if [[ -d "$repo_root/$workspace_dir" ]]; then
                workspaces=(${(f)"$(ls -1 "$repo_root/$workspace_dir" 2>/dev/null)"})
                _describe 'workspace' workspaces
              fi
            fi
          fi
          ;;
        create)
          _arguments \
            '-b[Use existing bookmark]:bookmark:->bookmarks' \
            '--bookmark[Use existing bookmark]:bookmark:->bookmarks' \
            '-r[Base revision]:revision:'
          if [[ $state == bookmarks ]]; then
            local bookmarks
            bookmarks=(${(f)"$(jj bookmark list --template 'name ++ "\n"' 2>/dev/null)"})
            _describe 'bookmark' bookmarks
          fi
          ;;
        init)
          local shells=(zsh bash fish)
          _describe 'shell' shells
          ;;
        cleanup)
          _arguments \
            '-f[Force deletion]' \
            '--force[Force deletion]' \
            '-k[Keep the associated bookmarks]' \
            '--keep-bookmark[Keep the associated bookmarks]' \
            '-n[Dry run - show what would be deleted]' \
            '--dry-run[Dry run - show what would be deleted]'
          ;;
        list)
          _arguments \
            '-v[Show detailed status]' \
            '--verbose[Show detailed status]'
          ;;
      esac
      ;;
  esac
}

_jjw_find_root() {
  local dir="$PWD"
  while [[ "$dir" != "/" ]]; do
    if [[ -f "$dir/.jjw.yaml" ]]; then
      echo "$dir"
      return
    fi
    dir=$(dirname "$dir")
  done
}

_jjw_workspace_dir() {
  local root="$1"
  local ws_dir
  ws_dir=$(command jjw config get workspace_dir 2>/dev/null)
  echo "${ws_dir:-workspaces}"
}

compdef _jjw jjw

jjw() {
  # Find repo root by walking up for .jjw.yaml
  local repo_root
  repo_root=$(_jjw_find_root)

  if [[ -z "$repo_root" ]]; then
    command jjw "$@"
    return $?
  fi

  case "$1" in
    create|delete|cleanup)
      local cdfile=$(mktemp)
      trap "rm -f '$cdfile'" EXIT
      JJW_CD_FILE="$cdfile" command jjw "$@"
      local exit_code=$?

      if [[ -s "$cdfile" ]]; then
        local target=$(cat "$cdfile")
        if [[ -d "$target" ]]; then
          cd "$target"
        fi
      fi
      rm -f "$cdfile"
      trap - EXIT
      return $exit_code
      ;;
    cd)
      local output
      output=$(command jjw "$@" 2>&1)
      local exit_code=$?

      if [[ $exit_code -eq 0 ]]; then
        local target="$output"
        if [[ -d "$target" ]]; then
          cd "$target"
        else
          echo "$output"
        fi
      else
        echo "$output"
      fi
      return $exit_code
      ;;
    exit)
      local target
      target=$(command jjw root 2>/dev/null)
      if [[ -d "$target" ]]; then
        cd "$target"
      else
        echo "Could not find repository root"
        return 1
      fi
      ;;
    *)
      command jjw "$@"
      ;;
  esac
}
`
}

// GenerateBash generates the bash shell integration script.
func GenerateBash() string {
	return `# jjw shell integration for bash
# Add this to your ~/.bashrc: eval "$(jjw init bash)"

_jjw_find_root() {
  local dir="$PWD"
  while [[ "$dir" != "/" ]]; do
    if [[ -f "$dir/.jjw.yaml" ]]; then
      echo "$dir"
      return
    fi
    dir=$(dirname "$dir")
  done
}

_jjw_workspace_dir() {
  local root="$1"
  local ws_dir
  ws_dir=$(command jjw config get workspace_dir 2>/dev/null)
  echo "${ws_dir:-workspaces}"
}

_jjw_completions() {
  local cur prev words cword
  _init_completion 2>/dev/null || {
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
  }

  local commands="create delete cd list cleanup exit init config root version help"

  if [[ $COMP_CWORD -eq 1 ]]; then
    COMPREPLY=($(compgen -W "$commands" -- "$cur"))
    return 0
  fi

  local cmd="${COMP_WORDS[1]}"
  case "$cmd" in
    cd|delete)
      local repo_root workspace_dir
      repo_root=$(_jjw_find_root)
      if [[ -n "$repo_root" ]]; then
        workspace_dir=$(_jjw_workspace_dir "$repo_root")
        if [[ -d "$repo_root/$workspace_dir" ]]; then
          COMPREPLY=($(compgen -W "$(ls -1 "$repo_root/$workspace_dir" 2>/dev/null)" -- "$cur"))
        fi
      fi
      ;;
    init)
      COMPREPLY=($(compgen -W "zsh bash fish" -- "$cur"))
      ;;
    cleanup)
      COMPREPLY=($(compgen -W "-n --dry-run -f --force -k --keep-bookmark" -- "$cur"))
      ;;
    list)
      COMPREPLY=($(compgen -W "-v --verbose" -- "$cur"))
      ;;
  esac
  return 0
}

complete -F _jjw_completions jjw

jjw() {
  local repo_root
  repo_root=$(_jjw_find_root)

  if [[ -z "$repo_root" ]]; then
    command jjw "$@"
    return $?
  fi

  case "$1" in
    create|delete|cleanup)
      local cdfile=$(mktemp)
      trap "rm -f '$cdfile'" EXIT
      JJW_CD_FILE="$cdfile" command jjw "$@"
      local exit_code=$?

      if [[ -s "$cdfile" ]]; then
        local target=$(cat "$cdfile")
        if [[ -d "$target" ]]; then
          cd "$target"
        fi
      fi
      rm -f "$cdfile"
      trap - EXIT
      return $exit_code
      ;;
    cd)
      local output
      output=$(command jjw "$@" 2>&1)
      local exit_code=$?

      if [[ $exit_code -eq 0 ]]; then
        local target="$output"
        if [[ -d "$target" ]]; then
          cd "$target"
        else
          echo "$output"
        fi
      else
        echo "$output"
      fi
      return $exit_code
      ;;
    exit)
      local target
      target=$(command jjw root 2>/dev/null)
      if [[ -d "$target" ]]; then
        cd "$target"
      else
        echo "Could not find repository root"
        return 1
      fi
      ;;
    *)
      command jjw "$@"
      ;;
  esac
}
`
}

// GenerateFish generates the fish shell integration script.
func GenerateFish() string {
	return `# jjw shell integration for fish
# Add this to your ~/.config/fish/config.fish: jjw init fish | source

complete -c jjw -f

complete -c jjw -n "__fish_use_subcommand" -a "create" -d "Create a new workspace"
complete -c jjw -n "__fish_use_subcommand" -a "delete" -d "Delete a workspace"
complete -c jjw -n "__fish_use_subcommand" -a "cd" -d "Change to a workspace directory"
complete -c jjw -n "__fish_use_subcommand" -a "list" -d "List all workspaces"
complete -c jjw -n "__fish_use_subcommand" -a "cleanup" -d "Clean up merged workspaces"
complete -c jjw -n "__fish_use_subcommand" -a "exit" -d "Return to the main repository"
complete -c jjw -n "__fish_use_subcommand" -a "init" -d "Generate shell integration script"
complete -c jjw -n "__fish_use_subcommand" -a "config" -d "Manage jjw configuration"
complete -c jjw -n "__fish_use_subcommand" -a "root" -d "Print the main repository root path"
complete -c jjw -n "__fish_use_subcommand" -a "version" -d "Print the version number"
complete -c jjw -n "__fish_use_subcommand" -a "help" -d "Help about any command"

function __jjw_find_root
  set -l dir $PWD
  while test "$dir" != "/"
    if test -f "$dir/.jjw.yaml"
      echo "$dir"
      return
    end
    set dir (dirname "$dir")
  end
end

function __jjw_workspaces
  set -l repo_root (__jjw_find_root)
  if test -z "$repo_root"
    return
  end
  set -l ws_dir (command jjw config get workspace_dir 2>/dev/null)
  test -z "$ws_dir"; and set ws_dir "workspaces"
  if test -d "$repo_root/$ws_dir"
    ls -1 "$repo_root/$ws_dir" 2>/dev/null
  end
end

complete -c jjw -n "__fish_seen_subcommand_from cd delete" -a "(__jjw_workspaces)"

complete -c jjw -n "__fish_seen_subcommand_from create" -s b -l bookmark -d "Use existing bookmark"
complete -c jjw -n "__fish_seen_subcommand_from create" -s r -l revision -d "Base revision"

complete -c jjw -n "__fish_seen_subcommand_from init" -a "zsh bash fish"

complete -c jjw -n "__fish_seen_subcommand_from delete" -s f -l force -d "Force deletion"
complete -c jjw -n "__fish_seen_subcommand_from delete" -s k -l keep-bookmark -d "Keep the associated bookmark"

complete -c jjw -n "__fish_seen_subcommand_from cleanup" -s n -l dry-run -d "Show what would be deleted"
complete -c jjw -n "__fish_seen_subcommand_from cleanup" -s f -l force -d "Skip confirmation"
complete -c jjw -n "__fish_seen_subcommand_from cleanup" -s k -l keep-bookmark -d "Keep the associated bookmarks"

complete -c jjw -n "__fish_seen_subcommand_from list" -s v -l verbose -d "Show detailed status"

function jjw
  set -l repo_root (__jjw_find_root)

  if test -z "$repo_root"
    command jjw $argv
    return $status
  end

  switch $argv[1]
    case create delete cleanup
      set -l cdfile (mktemp)
      JJW_CD_FILE="$cdfile" command jjw $argv
      set -l exit_code $status

      if test -s "$cdfile"
        set -l target (cat "$cdfile")
        if test -d "$target"
          cd "$target"
        end
      end
      rm -f "$cdfile"
      return $exit_code

    case cd
      set -l output (command jjw $argv 2>&1)
      set -l exit_code $status

      if test $exit_code -eq 0
        if test -d "$output"
          cd "$output"
        else
          echo "$output"
        end
      else
        echo "$output"
      end
      return $exit_code

    case exit
      set -l target (command jjw root 2>/dev/null)
      if test -d "$target"
        cd "$target"
      else
        echo "Could not find repository root"
        return 1
      end

    case '*'
      command jjw $argv
  end
end
`
}

// Generate returns the shell integration script for the given shell.
func Generate(shell string) (string, error) {
	switch shell {
	case "zsh":
		return GenerateZsh(), nil
	case "bash":
		return GenerateBash(), nil
	case "fish":
		return GenerateFish(), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s (supported: zsh, bash, fish)", shell)
	}
}
