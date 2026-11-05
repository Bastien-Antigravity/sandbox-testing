VERSION := $(shell cat VERSION.txt 2>/dev/null || echo "0.0.1")

.PHONY: all build test version clean

all: build

version:
	@echo $(VERSION)

build:
	@echo "Building repository (version $(VERSION))..."
	@if [ -f "go.mod" ]; then go build ./...; fi
	@if [ -f "Cargo.toml" ]; then cargo build --release; fi
	@if [ -f "setup.py" ] || [ -f "pyproject.toml" ]; then python3 -m build; fi

test:
	@echo "Running tests (version $(VERSION))..."
	@if [ -d "02-Scenarios/go" ] && [ -f "02-Scenarios/go/go.mod" ]; then (cd 02-Scenarios/go && go test -v -run "TestScenario_PolyglotToolingParity|TestScenario_DynamicPortShiftingAndZeroHardcodedFallbacks"); fi
	@if [ -f "go.mod" ]; then go test ./... || go test ./src/...; fi
	@if [ -f "Cargo.toml" ]; then cargo test; fi
	@if [ -f "requirements.txt" ] || [ -f "pyproject.toml" ]; then pytest; fi

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf dist build *.egg-info target/
