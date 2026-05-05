# Bastien-Antigravity Sandbox Testing
# Central Command for Scenario Validation

ORCHESTRATOR = python3 03-Orchestration/scenario_orchestrator.py
MODE ?= native

.PHONY: help list test-all clean

help:
	@echo "🌌 Antigravity Sandbox Testing Hub"
	@echo "Usage: make <target> [MODE=native|docker]"
	@echo ""
	@echo "Targets:"
	@echo "  list            : List available specifications"
	@echo "  test-all        : Run all validation scenarios"
	@echo "  clean           : Shutdown infrastructure and cleanup"
	@echo ""
	@echo "Scenarios:"
	@$(MAKE) -s list | sed 's/^/  /'

list:
	@find 01-Specifications -name "FEAT-*.yaml" -exec basename {} \;

test-all:
	@for f in $$(find 01-Specifications -name "FEAT-*.yaml"); do \
		echo "--- Running $$f ---"; \
		$(ORCHESTRATOR) $$f --mode $(MODE) || exit 1; \
	done

# Dynamic targets for specific features (e.g., make test-FEAT-000)
test-%:
	$(ORCHESTRATOR) 01-Specifications/$*.yaml --mode $(MODE)

clean:
	@echo "Cleaning up sandbox environment..."
	docker-compose -f 00-Environment/config/docker-compose.yaml down 2>/dev/null || true
	rm -rf 04-Reporting/*
