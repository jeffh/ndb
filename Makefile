.PHONY: clean test bench clean

GO := $(shell which go)
CMDS := $(shell ls cmd)

# Example goargs
# GOARGS=-race for race condition checking

all: $(CMDS)

# $(GO) build --ldflags="-s -w" $(GOARGS) -o ./bin/$@ ./cmd/$@
$(CMDS): $(find . -type '*.go')
	$(GO) build $(GOARGS) -o ./bin/$@ ./cmd/$@

test:
	$(GO) test $(GOARGS) ./...

clean:
	rm -rf bin; true
	go clean -cache

bench:
	$(GO) test $(GOARGS) -bench . -benchtime 5s -benchmem -cpuprofile=cpu.out -memprofile=mem.out -trace=trace.out
