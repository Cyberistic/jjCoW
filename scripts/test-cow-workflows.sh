#!/bin/sh
# End-to-end jj workflow tests for CoW workspaces.
# Requires: real jj, built bin/jjw, CoW-capable filesystem (APFS/Btrfs/XFS).
#
# Usage: scripts/test-cow-workflows.sh
set -eu

export JJ_CONFIG=/dev/null
export EDITOR=true
export VISUAL=true

JJW="$(pwd)/bin/jjw"
if [ ! -x "$JJW" ]; then
  echo "build bin/jjw first (just build)" >&2
  exit 1
fi

WORK="$(mktemp -d)/cow-workflows"
trap 'rm -rf "$(dirname "$WORK")"' EXIT
mkdir -p "$WORK"
cd "$WORK"

pass=0
fail=0
check() {
  name="$1"
  shift
  if sh -c "$*" >/dev/null 2>&1; then
    echo "  PASS  $name"
    pass=$((pass + 1))
  else
    echo "  FAIL  $name"
    fail=$((fail + 1))
  fi
}

echo "=== setup repo"
jj git init repo --colocate >/dev/null 2>&1
cd repo

printf 'hello\n' > README.md
printf 'workspaces/\n.env\n' > .gitignore
printf 'SECRET=1\n' > .env
jj describe -m "initial" >/dev/null 2>&1
jj bookmark create main -r @ >/dev/null 2>&1
jj new >/dev/null 2>&1
"$JJW" config init >/dev/null
# Commit .jjw.yaml so create doesn't always show it as A.
jj describe -m "jjw config" >/dev/null 2>&1
jj bookmark set main -r @ >/dev/null 2>&1
jj new >/dev/null 2>&1

# ── 1. CoW create: untracked carry-over, clean adoption ─────────────────────
echo "=== 1. cow create + adopt"
"$JJW" create feature-a >/dev/null 2>&1
check "workspace dir exists" 'test -d workspaces/feature-a'
check ".env carried over" 'test -f workspaces/feature-a/.env'
# .env is gitignored: on disk via CoW, but jj st stays clean for tracked files.
check "jj st clean (tracked)" 'cd workspaces/feature-a && jj st 2>&1 | grep -q "no changes"'
check "bookmark feature-a exists" 'jj bookmark list --template "name ++ \"\\n\"" | grep -qx feature-a'
check "parent is main" 'cd workspaces/feature-a && jj log -r @- --no-graph -T "bookmarks" | grep -q main'

# ── 2. New commit inside workspace ──────────────────────────────────────────
echo "=== 2. commit inside workspace"
(
  cd workspaces/feature-a
  printf 'feature\n' > feature.txt
  jj describe -m "add feature" >/dev/null 2>&1
  jj new >/dev/null 2>&1
)
check "workspace has feature.txt committed" 'cd workspaces/feature-a && jj log -r @- --no-graph -T "description" | grep -q "add feature"'
check "main unchanged" 'jj log -r main --no-graph -T "description" | grep -q "jjw config"'
check "feature.txt only in workspace tree" 'cd workspaces/feature-a && test -f feature.txt && cd ../.. && ! test -f feature.txt'

# ── 3. Merge workspace into main ────────────────────────────────────────────
echo "=== 3. merge workspace → main"
jj new main feature-a -m "merge feature-a" >/dev/null 2>&1
check "merge commit has feature.txt" 'test -f feature.txt'
jj bookmark set main -r @ >/dev/null 2>&1
jj new >/dev/null 2>&1
check "main includes merge" 'jj log -r main --no-graph -T "description" | grep -q "merge feature-a"'

# ── 4. Concurrent commits on main and workspace, then merge ─────────────────
echo "=== 4. concurrent commits + merge"
# Create workspace first (from current main), THEN diverge both sides.
"$JJW" create feature-b >/dev/null 2>&1
(
  cd workspaces/feature-b
  printf 'from-ws\n' > ws-only.txt
  jj describe -m "ws work" >/dev/null 2>&1
  jj new >/dev/null 2>&1
)
# Advance main independently (default workspace).
printf 'from-main\n' > main-only.txt
jj describe -m "main work" >/dev/null 2>&1
jj bookmark set main -r @ >/dev/null 2>&1
jj new >/dev/null 2>&1

check "ws has ws-only, not main-only (diverged)" 'cd workspaces/feature-b && test -f ws-only.txt && ! test -f main-only.txt'
check "main has main-only, not ws-only" 'test -f main-only.txt && ! test -f ws-only.txt'

jj new main feature-b -m "merge concurrent" >/dev/null 2>&1
check "merge has both files" 'test -f main-only.txt && test -f ws-only.txt'
jj bookmark set main -r @ >/dev/null 2>&1
jj new >/dev/null 2>&1

# ── 5. New bookmark inside workspace ────────────────────────────────────────
echo "=== 5. bookmark inside workspace"
(
  cd workspaces/feature-b
  jj bookmark create release-candidate -r @- >/dev/null 2>&1
)
check "bookmark created in workspace" 'jj bookmark list --template "name ++ \"\\n\"" | grep -qx release-candidate'
check "bookmark visible from main" 'jj bookmark list --template "name ++ \"\\n\"" | grep -qx release-candidate'

# ── 6. Edit detection after CoW create (tree_state is live) ─────────────────
echo "=== 6. live edit detection"
"$JJW" create feature-c >/dev/null 2>&1
(
  cd workspaces/feature-c
  printf 'mutated\n' > README.md
)
check "jj sees M README.md" 'cd workspaces/feature-c && jj st 2>&1 | grep -q "M README.md"'

# ── 7. Dirty source carries into new workspace ──────────────────────────────
echo "=== 7. dirty source carry-over"
printf 'dirty-main\n' > dirty.txt
jj st >/dev/null 2>&1
"$JJW" create feature-d >/dev/null 2>&1
check "dirty file present in new ws" 'test -f workspaces/feature-d/dirty.txt'
check "dirty shows as change in new ws" 'cd workspaces/feature-d && jj st 2>&1 | grep -qE "A dirty.txt|M dirty.txt"'
rm -f dirty.txt
jj st >/dev/null 2>&1

# ── 8. --lazy mode ──────────────────────────────────────────────────────────
echo "=== 8. lazy mode"
"$JJW" create feature-lazy --lazy >/dev/null 2>&1
check "lazy: files on disk" 'test -f workspaces/feature-lazy/README.md'
check "lazy: .env on disk" 'test -f workspaces/feature-lazy/.env'
(
  cd workspaces/feature-lazy
  jj sparse reset >/dev/null 2>&1
)
check "lazy: after sparse reset, st works" 'cd workspaces/feature-lazy && jj st >/dev/null 2>&1'

# ── 9. --no-cow still works ─────────────────────────────────────────────────
echo "=== 9. --no-cow"
"$JJW" create feature-nocow --no-cow >/dev/null 2>&1
check "nocow: workspace exists" 'test -d workspaces/feature-nocow'
check "nocow: no .env (untracked not carried)" '! test -f workspaces/feature-nocow/.env'
check "nocow: jj st clean" 'cd workspaces/feature-nocow && jj st 2>&1 | grep -q "no changes"'

# ── 10. delete CoW workspace ────────────────────────────────────────────────
echo "=== 10. delete"
"$JJW" delete feature-c --force >/dev/null 2>&1
check "deleted workspace gone" '! test -d workspaces/feature-c'
check "deleted bookmark gone" '! jj bookmark list --template "name ++ \"\\n\"" | grep -qx feature-c'

# ── 11. rebase workspace onto updated main ──────────────────────────────────
echo "=== 11. rebase workspace onto main"
"$JJW" create feature-e >/dev/null 2>&1
printf 'main-advance\n' > advance.txt
jj describe -m "advance main" >/dev/null 2>&1
jj bookmark set main -r @ >/dev/null 2>&1
jj new >/dev/null 2>&1
(
  cd workspaces/feature-e
  printf 'e-work\n' > e.txt
  jj describe -m "e work" >/dev/null 2>&1
  jj rebase -d main >/dev/null 2>&1
)
check "after rebase, has advance.txt" 'cd workspaces/feature-e && test -f advance.txt'
check "after rebase, has e.txt" 'cd workspaces/feature-e && test -f e.txt'

# ── 12. partial adoption (lazy tip) ─────────────────────────────────────────
echo "=== 12. partial sparse set --add"
"$JJW" create feature-partial --lazy >/dev/null 2>&1
(
  cd workspaces/feature-partial
  jj sparse set --add README.md >/dev/null 2>&1
)
check "partial adopt works" 'cd workspaces/feature-partial && jj st >/dev/null 2>&1'

echo ""
echo "=== results: $pass passed, $fail failed"
[ "$fail" -eq 0 ]
