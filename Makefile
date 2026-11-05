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

## lint-gocognit: Flag cognitive complexity above 30 in code changed since BASE (default origin/main).
BASE ?= origin/main
lint-gocognit:
	@echo "--> Running gocognit on changes since $(BASE)"
	@golangci-lint run --config .golangci-gocognit.yml --new-from-merge-base "$(BASE)"
.PHONY: lint-gocognit

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

## mutation: Run gremlins mutation testing on the root package. Set
## MUTATION_FILE=codecs.go to mutate only that file; the full test suite still
## runs against every mutant. Results are written to MUTATION_OUTPUT.
GREMLINS_VERSION ?= v0.6.0
GREMLINS ?= go run github.com/go-gremlins/gremlins/cmd/gremlins@$(GREMLINS_VERSION)
MUTATION_FILE ?=
MUTATION_OUTPUT ?= gremlins.json
mutation:
	@echo "--> Running gremlins mutation testing on $(or $(MUTATION_FILE),all files)"
	@set --; \
	if [ -n "$(MUTATION_FILE)" ]; then \
		[ -f "$(MUTATION_FILE)" ] || { echo "MUTATION_FILE=$(MUTATION_FILE) not found"; exit 1; }; \
		for f in *.go; do \
			case "$$f" in *_test.go|"$(MUTATION_FILE)") continue ;; esac; \
			set -- "$$@" -E "(^|/)$$(printf '%s' "$$f" | sed 's/\./\\./g')\$$"; \
		done; \
	fi; \
	$(GREMLINS) unleash "$$@" -o "$(MUTATION_OUTPUT)" .
.PHONY: mutation
