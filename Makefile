BIN_DIR=$(CURDIR)/bin
BENCH_FLAGS ?= -cpuprofile=cpu.pprof -memprofile=mem.pprof -benchmem
SITEMAPPER_DIR=./cmd/sitemapper
GOBIN_TEMP = $(CURDIR)/gobin
GOLINT = $(GOBIN_TEMP)/golint
STATICCHECK = $(GOBIN_TEMP)/staticcheck

GO_FILES := $(shell \
	find . '(' -path '*/.*' -o -path './vendor' ')' -prune \
	-o -name '*.go' -print | cut -b3-)

# modules that contains lintable code
MODULE_DIRS = . ./test/server

CMDS = ./cmd/sitemapper

.PHONY: all
all: lint test bench cover cmds

# Linting tools installed locally
$(GOBIN_TEMP):
	mkdir -p $(GOBIN_TEMP)

$(GOLINT): $(GOBIN_TEMP)
	GOBIN=$(GOBIN_TEMP) go install golang.org/x/lint/golint

$(STATICCHECK): $(GOBIN_TEMP)
	GOBIN=$(GOBIN_TEMP) go install honnef.co/go/tools/cmd/staticcheck

# Build commands
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

$(CMDS): $(BIN_DIR)
	go build -o $(BIN_DIR)/ $(realpath $@) 

.PHONY: cmds
cmds: $(CMDS)

.PHONY: lint
lint: $(GOLINT) $(STATICCHECK)
	@echo $(GO_FILES)
	@rm -rf lint.log
	@echo "Checking formatting..."
	@gofmt -d -s $(GO_FILES) 2>&1 | tee lint.log
	@echo "Checking vet..."
	@$(foreach dir,$(MODULE_DIRS),(cd $(dir) && go vet ./... 2>&1) &&) true | tee -a lint.log
	@echo "Checking lint..."
	@$(foreach dir,$(MODULE_DIRS),(cd $(dir) && $(GOLINT) ./... 2>&1) &&) true | tee -a lint.log
	@echo "Checking staticcheck..."
	@$(foreach dir,$(MODULE_DIRS),(cd $(dir) && $(STATICCHECK) ./... 2>&1) &&) true | tee -a lint.log
	@echo "Checking for unresolved FIXMEs..."
	@git grep -i fixme | grep -v -e Makefile | tee -a lint.log
	@echo "Checking for license headers..."
	@./checklicense.sh | tee -a lint.log
	@[ ! -s lint.log ]

.PHONY: test
test:
	go test -race

.PHONY: cover
cover:
	go test -race -coverprofile=cover.out -coverpkg=./
	go tool cover -html=cover.out -o cover.html

.PHONY: bench
BENCH ?= .
bench:
	go test -bench=$(BENCH) -run="^$$" $(BENCH_FLAGS)
