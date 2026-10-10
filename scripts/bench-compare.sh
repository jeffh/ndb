#!/usr/bin/env bash
# Compare the current tree against a base ref (default: origin/main) using
# git worktree + benchstat. Output is the standard go test -bench format so
# benchstat can consume it (-count=N).
set -euo pipefail

# Last x/perf revision whose go directive is 1.23 (this module's go.mod).
BENCHSTAT_PKG=golang.org/x/perf/cmd/benchstat@v0.0.0-20250807204132-c4b8702907f0

usage() {
  cat <<'EOF'
Compare HEAD (current working tree) against main via git worktree + benchstat.

Usage: scripts/bench-compare.sh [bench-regexp]

Environment:
  BASE       Base ref to compare (default: origin/main, then main)
  COUNT      go test -count (default: 6)
  BENCHTIME  go test -benchtime (default: 1s)
  SHORT      If 1, pass -short and skip 100MB+ (default: 1; forced to 0 when LARGE=1)
  LARGE      If 1, set NDB_BENCH_LARGE=1 and SHORT=0 (default: 0)
  OUTDIR     Output directory (default: <repo>/tmp/bench)
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
BENCH_RE="${1:-.}"
OUTDIR="${OUTDIR:-$ROOT/tmp/bench}"

if [[ "$LARGE" == "1" ]]; then
  if [[ "$SHORT" == "1" ]]; then
    echo "# LARGE=1 implies SHORT=0 (100MB+ would otherwise be skipped)" >&2
  fi
  SHORT=0
fi

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

dirty_header() {
  local dir="$1"
  if [[ -n "$(git -C "$dir" status --porcelain)" ]]; then
    echo "# dirty working tree: yes"
    git -C "$dir" status --porcelain | sed 's/^/#   /'
  else
    echo "# dirty working tree: no"
  fi
}

run_benches() {
  local dir="$1"
  local out="$2"
  (
    cd "$dir"
    echo "# dir=$dir"
    echo "# commit=$(git rev-parse --short HEAD) $(git log -1 --pretty=%s)"
    echo "# go=$(go env GOVERSION) $(go env GOOS)/$(go env GOARCH)"
    echo "# LARGE=$LARGE SHORT=$SHORT COUNT=$COUNT BENCHTIME=$BENCHTIME"
    dirty_header "$dir"
    go test "${GOTEST_ARGS[@]}"
  ) | tee "$out"
}

WORKTREE=""
cleanup() {
  if [[ -n "${WORKTREE:-}" && -d "$WORKTREE" ]]; then
    if ! git -C "$ROOT" worktree remove --force "$WORKTREE" >/dev/null 2>&1; then
      rm -rf "$WORKTREE"
      git -C "$ROOT" worktree prune
    fi
  fi
}
trap cleanup EXIT

BASE_SHA="$(git rev-parse "$BASE")"
WORKTREE="$(mktemp -d "${TMPDIR:-/tmp}/ndb-bench-XXXXXX")"
git worktree add --detach "$WORKTREE" "$BASE_SHA"

# Run HEAD's benchmark harness against the base library. Production files
# stay at $BASE; only *_test.go benches (and this script's peers) are overlaid
# so a new bench still compares main vs HEAD.
if [[ -f "$ROOT/bench_test.go" ]]; then
  cp "$ROOT/bench_test.go" "$WORKTREE/bench_test.go"
fi

echo "=== base ($BASE = $BASE_SHA, HEAD bench harness) ==="
run_benches "$WORKTREE" "$OLD_TXT"

echo "=== HEAD ($(git rev-parse --short HEAD), working tree) ==="
run_benches "$ROOT" "$NEW_TXT"

echo "=== benchstat ($OLD_TXT vs $NEW_TXT) ==="
go run "$BENCHSTAT_PKG" "$OLD_TXT" "$NEW_TXT" | tee "$OUTDIR/benchstat.txt"
