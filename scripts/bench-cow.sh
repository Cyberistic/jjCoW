#!/bin/sh
# Benchmark: jjw create modes
#   cow      - copy-on-write clone (default): clone + synchronous jj adoption
#   cow-lazy - copy-on-write clone with deferred adoption (--lazy)
#   nocow    - full jj checkout (--no-cow)
#
# Creates a throwaway jj repo, then times `jjw create` for N iterations in
# each mode, plus the disk space each workspace actually consumes.
#
# Usage: scripts/bench-cow.sh [iterations]
# Env:   BENCH_FILES (default 8000), BENCH_SIZE (bytes per file, default 125000)
set -eu

ITERATIONS="${1:-3}"
BENCH_FILES="${BENCH_FILES:-8000}"
BENCH_SIZE="${BENCH_SIZE:-125000}"
JJW="$(pwd)/bin/jjw"
WORK="$(mktemp -d)/bench"
REPO="$WORK/repo"

now() { perl -MTime::HiRes=time -e 'printf "%.3f\n", time'; }
used_kb() { df -k "$REPO" | tail -1 | awk '{print $3}'; }

trap 'rm -rf "$WORK"' EXIT

echo "=== setting up benchmark repo in $REPO ($BENCH_FILES files x $BENCH_SIZE bytes)"
mkdir -p "$REPO"
cd "$REPO"
jj git init --colocate >/dev/null 2>&1

i=0
while [ "$i" -lt "$BENCH_FILES" ]; do
  d="src/dir$((i % 100))"
  mkdir -p "$d"
  head -c "$BENCH_SIZE" /dev/urandom >"$d/file$i.bin"
  i=$((i + 1))
done

# untracked artifacts: node_modules (excluded by COW filter) and a .env (carried over)
# workspaces/ must be ignored so jj does not snapshot other workspaces
printf 'node_modules/\nworkspaces/\n' >.gitignore
echo 'SECRET=1' >.env
i=0
while [ "$i" -lt 100 ]; do
  d="node_modules/pkg$((i % 10))"
  mkdir -p "$d"
  head -c 10000 /dev/urandom >"$d/index$i.js"
  i=$((i + 1))
done

jj describe -m "bench base" >/dev/null 2>&1
jj bookmark create main -r @ >/dev/null 2>&1
jj new >/dev/null 2>&1
"$JJW" config init >/dev/null

echo "=== tracked working copy size:"
du -sh -I .jj -I .git . 2>/dev/null || du -sh .

run_mode() {
  mode="$1"
  shift
  echo "=== mode: $mode ($ITERATIONS iterations)"
  total=0
  min=999999
  max=0
  i=0
  while [ "$i" -lt "$ITERATIONS" ]; do
    before=$(used_kb)
    start=$(now)
    "$JJW" create "bench-$mode-$i" "$@" >/dev/null 2>&1
    end=$(now)
    sync
    after=$(used_kb)
    elapsed=$(echo "$end - $start" | bc)
    total=$(echo "$total + $elapsed" | bc)
    min=$(echo "if ($elapsed < $min) $elapsed else $min" | bc)
    max=$(echo "if ($elapsed > $max) $elapsed else $max" | bc)
    extra_mb=$(( (after - before) / 1024 ))
    printf "  create %-16s %7.2fs  (disk +%sMB)\n" "bench-$mode-$i" "$elapsed" "$extra_mb"
    i=$((i + 1))
  done
  avg=$(echo "scale=2; $total / $ITERATIONS" | bc)
  printf "  avg=%ss min=%ss max=%ss\n" "$avg" "$min" "$max"

  i=0
  while [ "$i" -lt "$ITERATIONS" ]; do
    "$JJW" delete "bench-$mode-$i" --force >/dev/null 2>&1
    i=$((i + 1))
  done
}

run_mode cow
run_mode cow-lazy --lazy
run_mode nocow --no-cow
