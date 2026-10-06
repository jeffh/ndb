.PHONY: all test bench clean

GO := $(shell which go)
CMDS := $(shell ls cmd)
SRCS := $(wildcard *.go) $(wildcard cmd/*/*.go)

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
