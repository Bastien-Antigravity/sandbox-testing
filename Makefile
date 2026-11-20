VERSION := $(strip $(shell cat VERSION.txt 2>/dev/null))
$(if $(VERSION),,$(error VERSION.txt is missing or empty))

.PHONY: all build test test-python verify-standalone version clean

all: build

version:
	@echo $(VERSION)

verify-standalone:
	@echo "Verifying standalone Go build (GOWORK=off)..."
	@if [ -d "02-Scenarios/go" ] && [ -f "02-Scenarios/go/go.mod" ]; then (cd 02-Scenarios/go && GOWORK=off go build ./...); fi

build:
	@echo "Building repository (version $(VERSION))..."
	@if [ -f "go.mod" ]; then go build ./...; fi
	@if [ -f "Cargo.toml" ]; then cargo build --release; fi
	@if [ -f "setup.py" ] || [ -f "pyproject.toml" ]; then python3 -m build; fi

test:
	@echo "Running tests (version $(VERSION))..."
	@if [ -d "02-Scenarios/go" ] && [ -f "02-Scenarios/go/go.mod" ]; then (cd 02-Scenarios/go && go test -v -run "TestScenario_PolyglotToolingParity|TestScenario_DynamicPortShiftingAndZeroHardcodedFallbacks"); fi
	@if [ -f "go.mod" ]; then go test -v ./...; fi
	@if [ -f "Cargo.toml" ]; then cargo test; fi
	@if [ -f "requirements.txt" ] || [ -f "pyproject.toml" ]; then pytest; fi

test-python:
	@echo "Running Python scenario tests (version $(VERSION))..."
	@if [ -d "02-Scenarios/python" ]; then pytest -v 02-Scenarios/python; fi

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf dist build *.egg-info target/
