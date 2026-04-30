#!/usr/bin/env python
# coding:utf-8

"""
ESSENTIAL PROCESS:
    Orchestrates test scenarios in either native (local binaries) or Docker modes.
    
DATA FLOW:
    1. Loads a YAML scenario file.
    2. Parses steps (start_service, wait, etc.).
    3. Executes commands via subprocess or docker-compose.
    4. Tracks running processes for cleanup.

KEY PARAMETERS:
    - scenario: Path to the YAML scenario file.
    - mode: 'native' or 'docker'.
    - root: Workspace root directory.
"""

from argparse import ArgumentParser as argparseArgumentParser
from yaml import safe_load as yamlSafeLoad
from subprocess import Popen as subprocessPopen, run as subprocessRun
from os.path import exists as osPathExists
from sys import exit as sysExit
from time import sleep as timeSleep
from pathlib import Path as pathlibPath
from typing import List, Any


class ScenarioRunner:
    Name = "ScenarioRunner"

    def __init__(self, mode: str, workspace_root: str, config: Any = None, logger: Any = None):
        self.mode = mode
        self.workspace_root = pathlibPath(workspace_root).resolve()
        self.processes: List[subprocessPopen] = []
        self.config = config
        self.logger = logger

    # -----------------------------------------------------------------------------------------------

    def log(self, msg: str) -> None:
        if self.logger and hasattr(self.logger, "info"):
            self.logger.info(f"{self.Name} : {msg}")
        else:
            print(f"[*] [{self.Name}] {msg}")

    # -----------------------------------------------------------------------------------------------

    def run_native(self, service_name: str, cmd_args: str) -> subprocessPopen:
        self.log(f"Starting service '{service_name}' in NATIVE mode...")
        # Assume binaries are built in cmd/<service-name>/<service-name>.exe or similar
        curr_dir = self.workspace_root / service_name
        self.log(f"CWD: {curr_dir}")
        
        try:
            process = subprocessPopen(cmd_args, cwd=str(curr_dir), shell=True)
            self.processes.append(process)
            return process
        except Exception as e:
            raise RuntimeError(f"Sandbox (Python): Failed to start native service '{service_name}': {e}")

    # -----------------------------------------------------------------------------------------------

    def run_docker(self, compose_file: str) -> None:
        self.log(f"Starting orchestration in DOCKER mode using {compose_file}...")
        try:
            subprocessRun(["docker-compose", "-f", compose_file, "up", "-d"], check=True)
        except Exception as e:
            raise RuntimeError(f"Sandbox (Python): Error starting docker-compose: {e}")

    # -----------------------------------------------------------------------------------------------

    def stop_all(self) -> None:
        self.log("Cleaning up resources...")
        if self.mode == "native":
            for p in self.processes:
                p.terminate()
        elif self.mode == "docker":
             # We might not want to stop everything automatically if the user wants to inspect logs
             pass

    # -----------------------------------------------------------------------------------------------

    def execute(self, scenario_path: str) -> None:
        if not osPathExists(scenario_path):
            raise FileNotFoundError(f"Sandbox (Python): Scenario file '{scenario_path}' not found")

        with open(scenario_path, "r") as f:
            scenario = yamlSafeLoad(f)

        self.log(f"Executing Scenario: {scenario.get('name', 'Unnamed Scenario')}")
        
        if self.mode == "docker":
            compose_path = self.workspace_root / "docker-deployment" / "docker-compose.yaml"
            self.run_docker(str(compose_path))
        else:
            for step in scenario.get("steps", []):
                action = step.get("action")
                if action == "start_service":
                    svc = step.get("service")
                    args = step.get("args", [])
                    self.run_native(svc, args)
                elif action == "wait":
                    duration = step.get("duration", 1)
                    self.log(f"Waiting for {duration} seconds...")
                    timeSleep(duration)

        self.log("Scenario execution complete.")


# ###################################################################################################
# MAIN EXECUTION
# ###################################################################################################

if __name__ == "__main__":
    parser = argparseArgumentParser(description="Antigravity Test Scenario Runner")
    parser.add_argument("scenario", help="Path to the YAML scenario file")
    parser.add_argument("--mode", choices=["native", "docker"], default="native", help="Execution mode")
    parser.add_argument("--root", default="..", help="Workspace root directory")
    
    args = parser.parse_args()
    
    # Initialize with dummy config/logger for standalone use
    runner = ScenarioRunner(args.mode, args.root, config={}, logger=None)
    try:
        runner.execute(args.scenario)
    except KeyboardInterrupt:
        runner.stop_all()
    except Exception as e:
        print(f"FATAL: {e}")
        sysExit(1)
    finally:
        # runner.stop_all() # Commented out to allow manual inspection for now
        pass

