---
title: "Project DNA: sandbox-testing"
microservice: sandbox-testing
type: infrastructure
status: active
---

# 🧬 Project DNA: sandbox-testing

## 🎯 High-Level Intent
- **Goal**: Provide a reliable, reproducible, and self-documenting "Chaos Lab" for the Bastien-Antigravity fleet.
- **Key Pattern**: **BDD-to-Implementation Binding**. We separate the "What" (Specifications) from the "How" (Technical Scenarios).

## 📐 Project Structure (2-Digit Standard)
- **00-Environment**: Infrastructure configurations.
- **01-Specifications**: Feature files (YAML).
- **02-Scenarios**: Language-specific test implementations.
- **03-Orchestration**: Management logic and scripts.
- **04-Reporting**: Audit logs and results.

## 🛠️ Role Specifics
- **Architect**: Maintain the orchestration script and infrastructure templates.
- **Developer**: Contribute feature-specific implementation logic in `02-Scenarios`.
- **Sentinel**: Review `04-Reporting` for ecosystem-wide regressions.

## 🚦 Lifecycle
- **Branching**: `main` is always stable.
- **Versioning**: Follows the fleet-wide semantic versioning.
