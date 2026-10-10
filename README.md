ndb
======

A Go library implementing the [Plan9 Network Database](https://9fans.github.io/plan9port/man/man7/ndb.html) format - a simple, human-readable key-value configuration file format.

### File Format

The ndb format is line-based with simple syntax:

```
# Comment
ip=192.168.1.1 sys=server01 dom=example.com
    # Continuation lines start with whitespace
    owner="John Doe"

# Attributes without values
sys=client01 dhcp
```

Key features:
- An array of records
- Continuation lines are indented for the same record
- Comments start with `#`
- Values with spaces use quotes: `attr="value with spaces"`
- Empty values allowed: just `attr` or `attr=`

## Installation

```bash
go get github.com/jeffh/ndb
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/jeffh/ndb"
)

func main() {
    // Open a database file
    db, err := ndb.Open(&ndb.LocalFileSystem{}, "hosts.ndb")
    if err != nil {
        panic(err)
    }

    // Search for records with specific attributes
    for rec := range db.Search(ndb.HasAttrValue("sys", "server01")) {
        if ip := rec.Get("ip"); ip != "" {
            fmt.Printf("IP: %s\n", ip)
        }
    }
}
```

## API Overview

### Opening Databases

```go
// Open a database (follows file references)
db, err := ndb.Open(&ndb.LocalFileSystem{}, "database.ndb")

// Open single file (no recursion)
db, err := ndb.OpenOne(&ndb.LocalFileSystem{}, "database.ndb")

// Parse in-memory data
db, err := ndb.ParseOneString("ip=192.168.1.1 sys=server01\n")
```

### Searching

The library provides iterator-based searching for efficient, lazy evaluation:

```go
// Search reuses one Record; copy it (rec.Copy()) to retain past the loop.
for rec := range db.Search(ndb.HasAttr("ip")) {
    fmt.Println(rec.Get("ip"))
}

// Search for specific attribute-value pairs
for rec := range db.Search(ndb.HasAttrValue("sys", "server01")) {
    // Process record
}

// Combine multiple predicates (AND logic)
for rec := range db.Search(ndb.HasAttr("ip"), ndb.HasAttrValue("sys", "server01")) {
    // Matches records with both conditions
}

// Get all results as a slice
records := db.SearchSlice(ndb.HasAttrValue("sys", "server01"))
```

### Working with Records

Records are collections of attribute-value pairs (tuples):

```go
// Get single value (first occurrence)
ip := rec.Get("ip")

// Get all values for an attribute
ips := rec.GetAll("ip")

// Check if attribute exists
if rec.HasKey("ip") {
    // ...
}

// Look up value with existence check
if val, ok := rec.Lookup("ip"); ok {
    fmt.Println(val)
}

// Iterate over tuples
for i := 0; i < rec.Len(); i++ {
    attr := rec.KeyAt(i)
    val := rec.ValueAt(i)
    fmt.Printf("%s=%s\n", attr, val)
}

// Convert to map (all values kept)
m := rec.AsMap()  // map[string][]string

// Get all keys/values
keys := rec.Keys()
values := rec.Values()
```

### Modifying Records

```go
// Pointer methods mutate in place
rec.Add(ndb.Tuple{Attr: "ip", Val: "192.168.1.1"})
rec.Put(ndb.Tuple{Attr: "ip", Val: "192.168.1.2"})
rec.Delete("ip", 0) // n=0 deletes all
rec.Replace("ip", "192.168.1.2", "192.168.1.3")
rec.ReplaceAt(0, ndb.Tuple{Attr: "ip", Val: "192.168.1.3"})
```

### File References

Database files can reference other files:

```
database file=hosts.ndb file=networks.ndb
```

The `Open()` function follows these references recursively; `OpenOne()` does not.

## Command-Line Tool

The library includes a query tool:

```bash
# Build
make

# Query all records
./bin/ndbq database.ndb

# Search by attribute
./bin/ndbq database.ndb ip

# Search by attribute=value
./bin/ndbq database.ndb sys server01
```

## Testing

```bash
# Run all tests
go test ./...

# Run with race detection
go test -race ./...
```

## Benchmarks

Regression benches cover the public search and iteration APIs:

| Benchmark | Public API | What it measures |
| --- | --- | --- |
| `BenchmarkSearchHasAttr` | `Search(HasAttr("sys"))` + `Get` | key-present search over every record |
| `BenchmarkSearchHasAttrMiss` | `Search(HasAttr("no-such-attr"))` | key scan with no matches (no record parse) |
| `BenchmarkSearchHasAttrValue` | `Search(HasAttrValue("sys", "bench-target"))` | key=value search for one known hit |
| `BenchmarkIterateAllRecords` | `All()` | iterate every record |
| `BenchmarkIterateRecordsAndKeys` | `All()` + `Len`/`KeyAt`/`ValueAt` | iterate every record and every key |

Fixtures are generated at bench time with a fixed seed into a process temp dir (one file per size, cached across calibration). They are never committed. A few percent of records use quoted values with spaces and `#` (`note="a b # c"`), trailing `#` comments, bare attributes, and varied attribute counts so the quoted-hash parse path is exercised.

Sizes:

- `small` — 64 KiB
- `medium` — 8 MiB (also reports peak memory)
- `large` — 128 MiB (peak memory; skipped unless `NDB_BENCH_LARGE=1`, and always skipped under `-short`)

```bash
# Default: small + medium. A normal `go test ./...` does not run benches.
go test -run '^$' -bench . -benchmem -count=6 -benchtime=1s

# Same, via make
make bench-stat

# Also run 128 MiB cases
NDB_BENCH_LARGE=1 go test -run '^$' -bench . -benchmem -count=3 -benchtime=1s

# Skip 100MB+ even if the env var is set
NDB_BENCH_LARGE=1 go test -short -run '^$' -bench . -benchmem
```

`b.SetBytes` is the fixture size, so the `MB/s` column is scan throughput. `b.ReportAllocs` reports `B/op` and `allocs/op`.

Published baseline tables in the PR that added these benches were taken on **Go 1.23**. CI and `go.mod` now use **Go 1.27** (`actions/setup-go` reads `go-version-file: go.mod`).

Medium and large cases also report custom metrics from a **separate** pass after the timed loop (`ReadMemStats` stops the world, so sampling inside the loop would inflate `ns/op`). Each metric is **peak minus a post-GC baseline** taken immediately before that pass, so the fixture already held in memory is not counted as scan growth:

- `peak-heap-B` — max `runtime.MemStats.HeapInuse` minus the post-GC baseline. This is the Go-level signal that `Search`/`All` started retaining every record instead of reusing one.
- `peak-rss-B` — max Linux RSS from `/proc/self/statm` minus the same baseline (resident pages × page size). **Linux-only; omitted on other OSes.** Includes non-Go mappings; treat it as process-footprint growth, not as `B/op`.

Compare the working tree to `origin/main` with [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) pinned in `scripts/bench-compare.sh`. The script creates a detached git worktree for the base ref and overlays this tree's `bench_test.go` onto it, so new benches still measure main's library. `LARGE=1` turns off `-short`.

```bash
make bench-compare
# or
COUNT=6 BENCHTIME=1s ./scripts/bench-compare.sh
# optional: LARGE=1 to include 128 MiB (implies SHORT=0)
```

An optional **Benchmarks** GitHub Actions workflow runs on `workflow_dispatch` and on pull requests that touch `**/*.go` (or the bench script/workflow). It is not part of the required push/PR checks, uses `contents: read` only, cancels overlapping runs on the same ref, uploads `bench.txt` as an artifact, and writes the same output to the job summary. The job is informational and is not used as a pass/fail perf gate. Default `-count` is 6, matching `scripts/bench-compare.sh`.
