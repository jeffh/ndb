.PHONY: clean

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
