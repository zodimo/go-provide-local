.PHONY: help test test-race bench bench-mem test-all

# Default target
help: ## Show this help message
	@echo "go-provide-local — compliance test suite"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  %-14s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

test: ## Run all tests
	go test ./plocal/...

test-race: ## Run all tests with the race detector
	go test -race ./plocal/...

bench: ## Run benchmarks (5 s per benchmark, no alloc reporting)
	go test -bench=. -benchtime=5s ./plocal/...

bench-mem: ## Run benchmarks with memory allocation reporting
	go test -bench=. -benchmem -benchtime=5s ./plocal/...

test-all: test-race bench-mem ## Run race-checked tests then full memory benchmarks
