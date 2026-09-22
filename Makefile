## build: Build the project.
build:
	@echo "--> Running go build"
	@go build ./...
.PHONY: build

## lint: Run golangci-lint.
lint:
	@echo "--> Running golangci-lint"
	@golangci-lint run
.PHONY: lint

## test: Run unit tests.
test:
	@echo "--> Running unit tests"
	@go test ./...
.PHONY: test

## fuzz: Run each fuzz target for FUZZTIME (default 30s).
FUZZTIME ?= 30s
fuzz:
	@echo "--> Running fuzz tests for $(FUZZTIME) each"
	@for target in $$(go test -list '^Fuzz' . | grep '^Fuzz'); do \
		echo "--> $$target"; \
		go test -run '^$$' -fuzz "^$$target$$" -fuzztime $(FUZZTIME) . || exit 1; \
	done
.PHONY: fuzz

## bench: Run benchmarks.
bench:
	@echo "--> Running benchmarks"
	@go test -benchmem -bench=.
.PHONY: bench
