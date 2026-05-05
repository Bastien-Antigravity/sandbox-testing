---
title: "Sandbox Orchestration Architecture"
type: architecture
status: active
microservice: sandbox-testing
---

# 🏛️ Sandbox Orchestration Architecture

## Overview
The Sandbox Testing Hub is designed to simulate complex interactions between microservices without requiring a full staging environment. It bridges the gap between Unit Tests and full System Integration.

## Key Components

### 1. The Scenario Orchestrator
The `scenario_orchestrator.py` is the brain of the hub. It interprets YAML specifications and manages the lifecycle of multiple services simultaneously.

- **Process Isolation**: It tracks PIDs for all native processes to ensure no "zombie" services remain after a test failure.
- **Environment Awareness**: It automatically switches between `localhost` and container-based networking depending on the `--mode` flag.

### 2. Multi-Mode Execution
- **Native Mode**: Faster feedback loop. Uses local binaries. Ideal for rapid development and debugging.
- **Docker Mode**: High reliability. Uses isolated containers and networks. Ideal for CI/CD and verifying network-level resilience.

### 3. Implementation Injection
Scenarios are decoupled from implementation. A YAML file in `01-Specifications` can point to any executable or test in `02-Scenarios`. This allows testing Go services with Python scripts or Rust validators seamlessly.

## Data Flow
1.  **Parse**: Load `01-Specifications/FEAT-XXX.yaml`.
2.  **Provision**: Start infrastructure (NATS, Config Server) via `00-Environment`.
3.  **Deploy**: Spawn the target microservices under test.
4.  **Validate**: Run the implementation steps in `02-Scenarios`.
5.  **Teardown**: Cleanup all resources.

## Best Practices
- **Idempotency**: Every test must be able to run in a loop without manual cleanup.
- **Self-Contained**: Avoid external dependencies. Use the `00-Environment` configurations.
- **Detailed Logging**: All output is routed to `04-Reporting` for post-mortem analysis.
