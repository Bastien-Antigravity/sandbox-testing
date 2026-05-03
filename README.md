---
microservice: sandbox-testing
type: repository
status: active
language: python
tags:
  - domain/testing
---

# Sandbox Testing: BDD-Driven Quality Assurance

This repository is the central hub for integration testing, behavior validation, and adversarial protocol hardening for the Bastien-Antigravity ecosystem. It follows a strict **BDD-Oriented** structure to separate feature definitions from technical implementations.

## 📂 Directory Structure

*   **`features/`**: High-level BDD scenario definitions (YAML). Each file is **bound** to a Business Spec via metadata headers.
*   **`implementations/`**: Polyglot code (Go, Python) that realizes the feature scenarios.
*   **`infra/`**: Infrastructure and orchestration plumbing.
    *   `config/`: Docker Compose and NATS configurations.
    *   `orchestrator/`: The core logic for the scenario runner.
*   **`bin/`**: Standard execution scripts and entry points.
*   **`results/`**: Standardized directory for test reports and artifacts.

## 🚀 Execution Guide

### 🛡️ QA Hardening Validation (Adversarial)

The sandbox includes a dedicated adversarial test suite to verify the security and resilience of the `log-server` protocol.

Run the hardening suite:
```bash
go test -v ./implementations/go/protocol_adversarial_test.go
```

### 🎮 Scenario Orchestration

The `scenario_orchestrator.py` tool manages the lifecycle of your tests across two primary modes:

1.  **Native Mode (`--mode native`)**: Runs binaries directly on your machine. Ideal for rapid development.
2.  **Docker Mode (`--mode docker`)**: Orchestrates the entire ecosystem using Docker Compose.

**Usage:**
```bash
# Run the Hello World scenario in Native mode
python infra/orchestrator/tools/scenario_orchestrator.py features/FEAT-000-hello-world.yaml --mode native

# Run the same scenario using Docker Compose
python infra/orchestrator/tools/scenario_orchestrator.py features/FEAT-000-hello-world.yaml --mode docker
```

## 🤖 AI Interaction Workflow

This sandbox is designed to work natively with the Antigravity AI assistant. You can request complex testing scenarios by referencing existing features.

**Example Prompts:**
*   *"Antigravity, generate a stress-test scenario in features/ for the universal-logger."*
*   *"Antigravity, implement a new Go step in implementations/go/ to verify the gRPC Log Bridge."*

The AI will generate YAML files in `features/` and implementations in `implementations/`, keeping the repository clean and scalable.
