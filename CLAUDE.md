# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

This is a Go library that implements the [Plan9 Network Database (ndb)](https://9fans.github.io/plan9port/man/man7/ndb.html) format. The ndb format is a simple key-value storage mechanism used for configuration information.

## Development Commands

### Testing
```bash
# Run all tests
go test ./...

# Run a single test
go test -run TestName

# Run tests with race detection
go test -race ./...

# Run benchmarks
go test -bench . -benchtime 5s -benchmem -cpuprofile=cpu.out -memprofile=mem.out -trace=trace.out
```

### Building
```bash
# Build all commands in cmd/
make

# Build with race detection
GOARGS=-race make

# Clean build artifacts
make clean
```

### Command-line Tool
```bash
# Build the ndbq query tool
make

# Query all records in a database
./bin/ndbq database.ndb

# Search for records with a specific attribute
./bin/ndbq database.ndb attribute

# Search for records with attribute=value
./bin/ndbq database.ndb attribute value
```

## Architecture

### Core Types

- **`Ndb`**: Main database type that manages file loading and searching
  - `Open(FileSystem, filepath)`: Opens database with recursive file resolution
  - `OpenOne(FileSystem, filepath)`: Opens single file without following references
  - `ParseOne([]byte)` / `ParseOneString(string)`: Parse in-memory data
  - Uses `FileSystem` interface for I/O abstraction

- **`Record`**: A slice of `Tuple` representing a single ndb record
  - Provides multiple accessor methods: `Get()`, `GetAll()`, `Lookup()`, `HasKey()`
  - Index-based access: `KeyAt()`, `ValueAt()`, `TupleAt()`
  - Mutation methods: `Add(Tuple)`, `Put(Tuple)`, `Delete(key, n)`, `Replace(key, old, new)` / `ReplaceAt(i, Tuple)`
  - Conversion: `AsMap()`, `Keys()`, `Values()`

- **`Tuple`**: Basic attribute-value pair with `Attr` and `Val` fields

- **`FileSystem` interface**: Abstraction for file I/O
  - `LocalFileSystem`: Real filesystem implementation
  - `MemoryFileSystem`: In-memory implementation for testing

### Search and Query API

The library uses iterator-based search with `iter.Seq[Record]`:

```go
// Iterator-based search (efficient, lazy)
for rec := range db.Search(HasAttrValue("key", "value")) {
    // Process record
}

// Slice-based search (loads all results into memory)
records := db.SearchSlice(HasAttrValue("key", "value"))

// Multiple predicates (AND logic)
db.Search(HasAttr("person"), HasAttrValue("name", "John"))
```

**Search Predicates**:
- `HasAttr(attr)`: Match records containing attribute
- `HasAttrValue(attr, val)`: Match records where attribute equals value

### File Format Details

- Records start with non-whitespace characters
- Continuation lines begin with whitespace
- Comments start with `#`
- Attribute-only: `attribute` (empty value)
- Attribute-value: `attribute=value`
- Quoted values: `attribute="value with spaces"`
- Database references: `database file="other.ndb" file="more.ndb"`

### Key Implementation Details

1. **Parsing Strategy**: The library parses on-demand during iteration rather than loading all records upfront. This is memory-efficient for large databases.

2. **Record Reuse**: The `Search()` iterator reuses a single `Record` slice internally. Callers must copy records if they need to retain them beyond the iteration.

3. **Recursive File Loading**: `Open()` follows database references, while `OpenOne()` does not. The `Open()` implementation loads files in multiple passes to handle recursive references.

## Testing Guidelines

- **Test cases should only use the Public API** - This is the primary project constraint
- Tests use `MemoryFileSystem` for in-memory testing
- Helper functions: `mustOpen(t, fs, path)`, `mustOpenOne(t, fs, path)`, `must(t, err)`
- Use `db.SearchSlice()` in tests for simpler assertions
- Example databases are embedded in `example/` directory using `//go:embed`
