#!/usr/bin/env bash
# Compare the current tree against a base ref (default: origin/main) using
# git worktree + benchstat. Output is the standard go test -bench format so
# benchstat can consume it (-count=N).
set -euo pipefail

usage() {
  cat <<'EOF'
Compare HEAD (current working tree) against main via git worktree + benchstat.

Usage: scripts/bench-compare.sh [bench-regexp]

Environment:
  BASE       Base ref to compare (default: origin/main, then main)
  COUNT      go test -count (default: 6)
  BENCHTIME  go test -benchtime (default: 1s)
  SHORT      If 1, pass -short and skip 100MB+ (default: 1)
  LARGE      If 1, set NDB_BENCH_LARGE=1 (default: 0)
  OUTDIR     Output directory (default: <repo>/tmp/bench)
  PKG        Package to test (default: .)
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

COUNT="${COUNT:-6}"
BENCHTIME="${BENCHTIME:-1s}"
SHORT="${SHORT:-1}"
LARGE="${LARGE:-0}"
PKG="${PKG:-.}"
BENCH_RE="${1:-.}"
OUTDIR="${OUTDIR:-$ROOT/tmp/bench}"

BASE="${BASE:-}"
if [[ -z "$BASE" ]]; then
  if git rev-parse --verify origin/main >/dev/null 2>&1; then
    BASE=origin/main
  else
    BASE=main
  fi
fi

mkdir -p "$OUTDIR"
OLD_TXT="$OUTDIR/old.txt"
NEW_TXT="$OUTDIR/new.txt"

GOTEST_ARGS=(-run '^$' -bench "$BENCH_RE" -benchmem -count="$COUNT" -benchtime="$BENCHTIME")
if [[ "$SHORT" == "1" ]]; then
  GOTEST_ARGS+=(-short)
fi

export NDB_BENCH_LARGE=""
if [[ "$LARGE" == "1" ]]; then
  export NDB_BENCH_LARGE=1
fi

run_benches() {
  local dir="$1"
  local out="$2"
  (
    cd "$dir"
    echo "# dir=$dir"
    echo "# commit=$(git rev-parse --short HEAD) $(git log -1 --pretty=%s)"
    echo "# go=$(go env GOVERSION) $(go env GOOS)/$(go env GOARCH)"
    go test "${GOTEST_ARGS[@]}"
  ) | tee "$out"
}

WORKTREE=""
cleanup() {
  if [[ -n "$WORKTREE" && -d "$WORKTREE" ]]; then
    git -C "$ROOT" worktree remove --force "$WORKTREE" >/dev/null 2>&1 || rm -rf "$WORKTREE"
  fi
}
trap cleanup EXIT

BASE_SHA="$(git rev-parse "$BASE")"
WORKTREE="$(mktemp -d "${TMPDIR:-/tmp}/ndb-bench-XXXXXX")"
git worktree add --detach "$WORKTREE" "$BASE_SHA"

echo "=== base ($BASE = $BASE_SHA) ==="
run_benches "$WORKTREE" "$OLD_TXT"

echo "=== HEAD ($(git rev-parse --short HEAD), working tree) ==="
run_benches "$ROOT" "$NEW_TXT"

run_benchstat() {
  if command -v benchstat >/dev/null 2>&1; then
    benchstat "$@"
  else
    go run golang.org/x/perf/cmd/benchstat@latest "$@"
  fi
}

echo "=== benchstat ($OLD_TXT vs $NEW_TXT) ==="
run_benchstat "$OLD_TXT" "$NEW_TXT" | tee "$OUTDIR/benchstat.txt"
