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

## bench: Run benchmarks.
bench:
	@echo "--> Running benchmarks"
	@go test -benchmem -bench=.
.PHONY: bench
