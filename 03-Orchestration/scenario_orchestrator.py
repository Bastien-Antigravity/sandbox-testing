#!/usr/bin/env python3
# coding:utf-8

"""
CORE PROCESS:
    Orchestrates complex test scenarios across the Bastien-Antigravity fleet.
    Supports both 'native' (local process) and 'docker' execution modes.

DIAGRAM:
    [YAML Spec] -> [ScenarioRunner] -> [Start Services] -> [Run Implementation Tests] -> [Cleanup]
"""

import os
import sys
import time
import subprocess
import yaml
from argparse import ArgumentParser
from pathlib import Path
from typing import List, Any, Optional

class ScenarioRunner:
    """Handles the lifecycle of a test scenario."""
    
    def __init__(self, mode: str, workspace_root: str):
        self.mode = mode
        self.workspace_root = Path(workspace_root).resolve()
        self.sandbox_root = self.workspace_root / "sandbox-testing"
        self.processes: List[subprocess.Popen] = []
        self.active_containers = False

    def log(self, msg: str, level: str = "INFO"):
        print(f"[{level}] [Orchestrator] {msg}")

    def run_native(self, service_name: str, cmd_args: Any) -> subprocess.Popen:
        self.log(f"Spawning native service: {service_name}")
        curr_dir = self.workspace_root / service_name
        
        if isinstance(cmd_args, list):
            cmd_args = " ".join(cmd_args)

        try:
            process = subprocess.Popen(cmd_args, cwd=str(curr_dir), shell=True)
            self.processes.append(process)
            return process
        except Exception as e:
            self.log(f"Failed to start native service '{service_name}': {e}", "ERROR")
            raise

    def run_docker(self):
        compose_path = self.sandbox_root / "00-Environment" / "config" / "docker-compose.yaml"
        if not compose_path.exists():
            self.log(f"Docker Compose file not found: {compose_path}", "ERROR")
            return

        self.log(f"Spinning up Docker infrastructure: {compose_path.name}")
        try:
            subprocess.run(["docker-compose", "-f", str(compose_path), "up", "-d"], check=True)
            self.active_containers = True
        except Exception as e:
            self.log(f"Docker initialization failed: {e}", "ERROR")
            raise

    def run_implementation(self, step_path: str):
        """Executes the technical validation logic (e.g. Go tests)."""
        self.log(f"Executing validation logic: {step_path}")
        
        # Format: 02-Scenarios/go/test.go::TestName
        parts = step_path.split("::")
        # Handle cases where user might have used old 'implementations' path
        rel_path = parts[0].replace("implementations/", "02-Scenarios/")
        file_path = self.sandbox_root / rel_path
        
        if not file_path.exists():
            self.log(f"Implementation file not found at {file_path}", "ERROR")
            return

        try:
            if file_path.suffix == ".go":
                cmd = ["go", "test", "-v", str(file_path)]
                if len(parts) > 1:
                    cmd.extend(["-run", parts[1]])
                subprocess.run(cmd, cwd=str(file_path.parent), check=True)
            elif file_path.suffix == ".py":
                subprocess.run([sys.executable, str(file_path)], check=True)
        except subprocess.CalledProcessError as e:
            self.log(f"Validation failed: {e}", "ERROR")
            raise

    def stop_all(self):
        self.log("Cleaning up resources...")
        for p in self.processes:
            try:
                p.terminate()
                p.wait(timeout=5)
            except:
                p.kill()
        
        if self.active_containers:
            compose_path = self.sandbox_root / "00-Environment" / "config" / "docker-compose.yaml"
            subprocess.run(["docker-compose", "-f", str(compose_path), "down"], check=False)

    def execute(self, scenario_path: str):
        scenario_file = Path(scenario_path)
        if not scenario_file.exists():
            # Try looking in 01-Specifications
            scenario_file = self.sandbox_root / "01-Specifications" / scenario_path.split("/")[-1]
            
        if not scenario_file.exists():
            raise FileNotFoundError(f"Scenario file not found: {scenario_path}")

        with open(scenario_file, "r") as f:
            scenario = yaml.safe_load(f)

        self.log(f"🚀 Starting Scenario: {scenario.get('name', 'Unnamed')}")
        
        if self.mode == "docker":
            self.run_docker()
        
        steps = scenario.get("steps", []) or scenario.get("scenario", [])
        for step in steps:
            action = step.get("action")
            if action == "start_service":
                self.run_native(step.get("service"), step.get("args", ""))
            elif action == "wait":
                duration = step.get("duration", 1)
                self.log(f"Waiting {duration}s...")
                time.sleep(duration)
            
            impl = step.get("step")
            if impl:
                self.run_implementation(impl)

        self.log("✅ Scenario completed successfully.")

if __name__ == "__main__":
    parser = ArgumentParser(description="Bastien-Antigravity Scenario Orchestrator")
    parser.add_argument("scenario", help="Path to YAML scenario")
    parser.add_argument("--mode", choices=["native", "docker"], default="native")
    parser.add_argument("--root", default="..", help="Workspace root")
    
    args = parser.parse_args()
    runner = ScenarioRunner(args.mode, args.root)
    
    try:
        runner.execute(args.scenario)
    except KeyboardInterrupt:
        runner.log("Interrupted by user.")
        runner.stop_all()
    except Exception as e:
        runner.log(f"Fatal error: {e}", "CRITICAL")
        runner.stop_all()
        sys.exit(1)
    finally:
        # Keep resources alive in native mode for inspection if desired, 
        # but docker should probably stay up or down based on preference.
        pass
