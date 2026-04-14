import argparse
import yaml
import subprocess
import os
import sys
import time
from pathlib import Path

class ScenarioRunner:
    def __init__(self, mode, workspace_root):
        self.mode = mode
        self.workspace_root = Path(workspace_root).resolve()
        self.processes = []

    def log(self, msg):
        print(f"[*] [ScenarioRunner] {msg}")

    def run_native(self, service_name, cmd_args):
        self.log(f"Starting service '{service_name}' in NATIVE mode...")
        # Assume binaries are built in cmd/<service-name>/<service-name>.exe or similar
        # For simplicity, we'll try to follow the bastien_make.py logic or assume they are in path
        curr_dir = self.workspace_root / service_name
        self.log(f"CWD: {curr_dir}")
        
        # This is a simplified execution. In a real scenario, we might need to build first.
        process = subprocess.Popen(cmd_args, cwd=str(curr_dir), shell=True)
        self.processes.append(process)
        return process

    def run_docker(self, compose_file):
        self.log(f"Starting orchestration in DOCKER mode using {compose_file}...")
        try:
            subprocess.run(["docker-compose", "-f", compose_file, "up", "-d"], check=True)
        except Exception as e:
            self.log(f"Error starting docker-compose: {e}")
            sys.exit(1)

    def stop_all(self):
        self.log("Cleaning up resources...")
        if self.mode == "native":
            for p in self.processes:
                p.terminate()
        elif self.mode == "docker":
             # We might not want to stop everything automatically if the user wants to inspect logs
             pass

    def execute(self, scenario_path):
        with open(scenario_path, "r") as f:
            scenario = yaml.safe_load(f)

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
                    time.sleep(duration)

        self.log("Scenario execution complete.")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Antigravity Test Scenario Runner")
    parser.add_argument("scenario", help="Path to the YAML scenario file")
    parser.add_argument("--mode", choices=["native", "docker"], default="native", help="Execution mode")
    parser.add_argument("--root", default="..", help="Workspace root directory")
    
    args = parser.parse_args()
    
    runner = ScenarioRunner(args.mode, args.root)
    try:
        runner.execute(args.scenario)
    except KeyboardInterrupt:
        runner.stop_all()
    finally:
        # runner.stop_all() # Commented out to allow manual inspection for now
        pass
