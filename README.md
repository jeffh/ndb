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

# Run benchmarks
go test -bench . -benchmem
```
