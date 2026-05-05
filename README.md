---
title: "Sandbox Testing Hub"
type: repository
status: active
microservice: sandbox-testing
---

# 🧪 Sandbox Testing Hub

The central command for validation, resilience testing, and chaos engineering across the Bastien-Antigravity fleet.

## 📐 Project Structure (The 2-Digit Standard)

This repository follows a structured boundary system to ensure predictable navigation and high maintainability:

- **`00-Environment`**: Infrastructure as Code (Docker Compose, NATS configurations, and network topologies).
- **`01-Specifications`**: BDD-style YAML files defining the "What" (Business Scenarios).
- **`02-Scenarios`**: Technical implementations in Go, Rust, or Python defining the "How" (Validation Logic).
- **`03-Orchestration`**: Management scripts and the `scenario_orchestrator.py` that ties everything together.
- **`04-Reporting`**: Centralized output for logs, test results, and performance audit trails.

## 🚀 Quick Start

### 1. List available scenarios
```bash
make list
```

### 2. Run a specific scenario (Native Mode)
```bash
make test-FEAT-000-hello-world
```

### 3. Run all scenarios in Docker Mode
```bash
make test-all MODE=docker
```

## 🛠️ Adding New Tests

1.  **Define the Specification**: Create a new `FEAT-XXX.yaml` in `01-Specifications/`.
2.  **Implement the Logic**: Add the corresponding test code in `02-Scenarios/<language>/`.
3.  **Validate**: Run your scenario using the `Makefile`.

## 🛡️ Reliability Guarantees
- **Atomic Cleanup**: The orchestrator ensures that all spawned processes and containers are terminated on failure or completion.
- **Environment Isolation**: Support for Docker-mode ensures that tests run in a clean, reproducible network environment.
- **Shared Memory Testing**: Specific support for high-throughput testing via shared-memory and safe-socket protocols.
