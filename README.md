---
microservice: sandbox-testing
type: repository
status: active
language: python
tags:
  - domain/testing
---

# Sandbox Testing: AI-Driven Quality Assurance

This repository is the central hub for integration testing, behavior validation, and AI-generated scenarios for the Bastien-Antigravity ecosystem.

## 🚀 Getting Started

The sandbox supports two primary execution modes for your testing scenarios:

### 🎮 Execution Modes

1.  **Native Mode (`--mode native`)**: Runs individual service binaries directly on your machine. Ideal for rapid development and debugging. It requires that your `go.work` and local builds are correctly configured.
2.  **Docker Mode (`--mode docker`)**: Orchestrates the entire ecosystem using the configurations in `docker-deployment`. Ideal for verifying production-like networking and service discovery.

---

## 🛠️ Tools

*   **`tools/bastien_scenario.py`**: The multi-mode scenario runner. It parses behavioral specs from the `scenarios/` directory and orchestrates the environment.

### Usage
```powershell
# Run the Hello World scenario in Native mode
python tools/bastien_scenario.py scenarios/hello_world.yaml --mode native

# Run the same scenario using Docker Compose
python tools/bastien_scenario.py scenarios/hello_world.yaml --mode docker
```

---

## 🤖 AI Interaction Workflow

This sandbox is designed to work natively with the Antigravity AI assistant. You can request complex testing scenarios by referencing existing documentation and features.

**Example Prompts:**
*   *"Antigravity, analyze the universal-logger docs and generate a stress-test scenario in the sandbox."*
*   *"Antigravity, create a scenario that verifies the gRPC lifecycle methods (Start/Stop) for the config-server."*

The AI will generate a YAML file in `scenarios/` which you can then execute using the `bastien_scenario.py` tool.
