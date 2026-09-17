# AGENTS.md: sandbox-testing

## Service Mission & Architecture Role
`sandbox-testing` provides end-to-end integration environments, mock service providers, and deterministic scenario runners for validating multi-service orchestration across the Bastien-Antigravity fleet without requiring live production external dependencies.

- **Ecosystem Role**: Integration test orchestration and mock harness.
- **Structure**:
  - `00-Environment`: Sandbox environment configuration
  - `01-Specifications`: Behavior specifications and contracts
  - `02-Scenarios`: End-to-end test scenarios
  - `03-Orchestration`: Execution harnesses
  - `04-Mock-Provider`: Mock daemons (NATS, Telegram, gRPC mock sinks)
- **Configuration Link**: `standalone.yaml -> ../docker-deployment/modes/local/config/native.yaml`

## Key Commands
```bash
# Run sandbox scenario suite
make test
```

## AI Development & Integration Guidelines
1. **Mock Isolation**: Ensure mock services bind to localhost test ports and do not interfere with running development daemons.
2. **Deterministic Cleanup**: All sandbox runs must clean up temporary sockets, ports, and state upon completion or failure.
3. **Header Ritual**: Source files MUST contain the Triple-Block header (`ESSENTIAL PROCESS`, `DATA FLOW`, `KEY PARAMETERS`).
