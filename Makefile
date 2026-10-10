.PHONY: all test bench bench-stat bench-compare clean

GO := $(shell which go)
CMDS := $(shell ls cmd)
SRCS := $(wildcard *.go) $(wildcard cmd/*/*.go)
COUNT ?= 6
BENCHTIME ?= 1s

# Example goargs
# GOARGS=-race for race condition checking

all: $(CMDS)

$(CMDS): $(SRCS)
	$(GO) build $(GOARGS) -o ./bin/$@ ./cmd/$@

test:
	$(GO) test $(GOARGS) ./...

clean:
	rm -rf ./bin

bench:
	$(GO) test $(GOARGS) -bench . -benchtime 5s -benchmem -cpuprofile=cpu.out -memprofile=mem.out -trace=trace.out

# Benchstat-friendly run (no profiles). Skips 100MB+ unless NDB_BENCH_LARGE=1.
bench-stat:
	$(GO) test $(GOARGS) -run '^$$' -bench . -benchmem -count=$(COUNT) -benchtime=$(BENCHTIME)

# Compare the working tree against origin/main (git worktree + benchstat).
bench-compare:
	./scripts/bench-compare.sh
