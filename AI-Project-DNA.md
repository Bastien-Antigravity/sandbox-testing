# 🧬 Project DNA: sandbox-testing

## 🎯 High-Level Intent (BDD)
- **Goal**: Provide a centralized, multi-mode environment for executing integration tests and validating system-wide behavior across the Bastien-Antigravity ecosystem.
- **Key Pattern**: **Multi-Mode Orchestration** (Native vs Docker) and **Scenario-Based Testing** (YAML definitions).
- **Behavioral Source of Truth**: [[business-bdd-brain/02-Behavior-Specs/sandbox-testing]]

## 🛠️ Role Specifics
- **Architect**: 
    - Ensure that the orchestration logic is decoupled from individual microservice implementations.
    - Maintain clean state resets between test scenarios.
- **QA**: 
    - Design complex, multi-service scenarios that verify edge cases (e.g., partial failure, latency injection).
    - Ensure that all AI-generated scenarios in `scenarios/` follow the standardized YAML schema.
- **Developer**:
    - Add new "Orchestrator Plugins" for new microservice types.
    - Follow the Python coding rules for all tools in `tools/`.

## 🚦 Lifecycle & Versioning
- **Primary Branch**: `develop`
- **Protected Branches**: `main`, `master`
- **Versioning Strategy**: Semantic Versioning (vX.Y.Z).
- **Version Source of Truth**: `VERSION.txt`.
