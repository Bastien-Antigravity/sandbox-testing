#!/bin/bash
set -e

# 1. Build and Start the standard sandbox (if not already running)
echo ">>> Starting the microservices sandbox..."
docker compose --profile test up -d --build

# 2. Run the orchestrator to execute the resilience scenario
echo ">>> Running the resilience scenario (Config Server)..."
docker compose run --rm test-orchestrator

# 3. Cleanup (optional)
# docker compose down
