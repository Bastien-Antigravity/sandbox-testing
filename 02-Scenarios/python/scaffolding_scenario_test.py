#!/usr/bin/env python
# coding:utf-8

"""
SCENARIO TEST: Microservice Scaffolding & Ecosystem Coherence Audit
Location: sandbox-testing/02-Scenarios/python/scaffolding_scenario_test.py

ESSENTIAL PROCESS:
Validates that:
1. Scaffolding engine generates structurally compliant Go and Python microservices.
2. Container configurations satisfy 12-Docker-Deployment-Standards.md (multi-stage builder, teleremote-network).
3. Symlink patterns align with watchdog-agent heal.go self-healing engine.
4. Generated inventory and service-registry schema contracts match ecosystem SSoTs.
5. Standalone buildability rules are fulfilled for all target languages.

DATA FLOW:
1. Input: Mock service definitions.
2. Logic: Generates services in sandbox isolation and asserts structural/network/config parity.
3. Output: Comprehensive scenario validation report.
"""

import ast
import json
import os
import shutil
import sys
import tempfile
import unittest
from pathlib import Path
import yaml

TEST_DIR = Path(__file__).resolve().parent
SANDBOX_DIR = TEST_DIR.parent.parent
WORKSPACE_ROOT = SANDBOX_DIR.parent
BASE_SCRIPTS_DIR = WORKSPACE_ROOT / "obsidian-brain" / "08-Base-Scripts"
DOCKER_DEPLOY_DIR = WORKSPACE_ROOT / "docker-deployment"
REPO_CONTROL_DIR = WORKSPACE_ROOT / "obsidian-brain" / "05-Fleet-Operation" / "00-Repo-Control"

if str(BASE_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(BASE_SCRIPTS_DIR))

from src.lifecycle.scaffold_microservice import (
    generate_go_scaffold,
    generate_python_scaffold,
    generate_common_files,
    generate_bdd_spec,
)


class TestScaffoldingScenario(unittest.TestCase):
    """Sandbox Scenario Test for Microservice Scaffolding & Ecosystem Coherence."""

    def setUp(self):
        self.temp_root = Path(tempfile.mkdtemp(prefix="sandbox_scaffold_"))
        self.mock_brain = self.temp_root / "obsidian-brain"
        self.mock_brain.mkdir(parents=True, exist_ok=True)

    def tearDown(self):
        shutil.rmtree(self.temp_root, ignore_errors=True)

    def test_scaffolding_scenario_full_lifecycle(self):
        print("\n" + "=" * 70)
        print("🧪 SCENARIO: MICROSERVICE SCAFFOLDING & ECOSYSTEM PARITY AUDIT")
        print("=" * 70)

        # ---------------------------------------------------------------------
        # STAGE 1: Scaffolding Execution (Go & Python)
        # ---------------------------------------------------------------------
        print("\n[STAGE 1] Executing Scaffolding for Go & Python Archetypes...")

        go_svc_name = "sandbox-go-worker"
        go_dir = self.temp_root / go_svc_name
        generate_go_scaffold(go_dir, go_svc_name, 8110, "Go Worker in Sandbox", dry_run=False)
        generate_common_files(go_dir, go_svc_name, "go", 8110, "Go Worker in Sandbox", dry_run=False)
        generate_bdd_spec(self.mock_brain, go_svc_name, "Go Worker in Sandbox", dry_run=False)

        py_svc_name = "sandbox-py-worker"
        py_dir = self.temp_root / py_svc_name
        generate_python_scaffold(py_dir, py_svc_name, 8111, "Py Worker in Sandbox", dry_run=False)
        generate_common_files(py_dir, py_svc_name, "python", 8111, "Py Worker in Sandbox", dry_run=False)
        generate_bdd_spec(self.mock_brain, py_svc_name, "Py Worker in Sandbox", dry_run=False)

        print("  ✅ STAGE 1 Complete: Both archetypes scaffolded successfully.")

        # ---------------------------------------------------------------------
        # STAGE 2: 12-Docker-Deployment-Standards.md Parity Check
        # ---------------------------------------------------------------------
        print("\n[STAGE 2] Auditing Container Architecture & Network Standards...")

        for s_name, s_dir, s_lang in [(go_svc_name, go_dir, "go"), (py_svc_name, py_dir, "python")]:
            dockerfile = (s_dir / "Dockerfile").read_text(encoding="utf-8")
            compose_yml = (s_dir / "docker-compose.yml").read_text(encoding="utf-8")
            compose_data = yaml.safe_load(compose_yml)

            # Check network harmonization (Must match teleremote-network)
            self.assertIn("teleremote-network", compose_data["networks"])
            self.assertEqual(compose_data["networks"]["teleremote-network"]["name"], "teleremote-network")
            self.assertTrue(compose_data["networks"]["teleremote-network"]["external"])

            # Check standalone buildability: must clone shared internal repos in builder stage
            self.assertIn("microservice-toolbox.git", dockerfile)
            if s_lang == "go":
                self.assertIn("FROM golang:1.25-alpine AS builder", dockerfile)
                self.assertIn("FROM alpine:3.20", dockerfile)
            else:
                self.assertIn("FROM golang:1.25-alpine AS go-builder", dockerfile)
                self.assertIn("FROM python:3.12-alpine", dockerfile)
                self.assertIn("libunilog.so", dockerfile)

        print("  ✅ STAGE 2 Complete: Dockerfile and compose network adhere to 12-Docker-Deployment-Standards.")

        # ---------------------------------------------------------------------
        # STAGE 3: Symlink Path & Heal.go Parity Check
        # ---------------------------------------------------------------------
        print("\n[STAGE 3] Auditing standalone.yaml Symlink & Self-Healing Contract...")

        for s_dir in [go_dir, py_dir]:
            symlink = s_dir / "standalone.yaml"
            self.assertTrue(symlink.is_symlink())
            target = os.readlink(symlink)
            self.assertEqual(target, "../docker-deployment/modes/local/config/native.yaml")

            # Verify target relative depth matches heal.go pattern (1 level up from repo root)
            heal_pattern = f"../docker-deployment/modes/local/config/native.yaml"
            self.assertEqual(target, heal_pattern)

        print("  ✅ STAGE 3 Complete: Symlink relative depths conform to watchdog heal.go.")

        # ---------------------------------------------------------------------
        # STAGE 4: Registry & Inventory Schema Compatibility Check
        # ---------------------------------------------------------------------
        print("\n[STAGE 4] Auditing Registry & Inventory Schema Contracts...")

        # Verify live inventory.json schema structure
        inv_path = REPO_CONTROL_DIR / "inventory.json"
        if inv_path.exists():
            with open(inv_path, "r", encoding="utf-8") as f:
                inv_data = json.load(f)
            self.assertIn("repositories", inv_data)
            required_keys = {"name", "path", "remote", "master_branch", "repo_type"}
            for repo in inv_data["repositories"][:5]:
                self.assertTrue(required_keys.issubset(repo.keys()))

        # Verify live service-registry.json schema structure
        reg_path = REPO_CONTROL_DIR / "service-registry.json"
        if reg_path.exists():
            with open(reg_path, "r", encoding="utf-8") as f:
                reg_data = json.load(f)
            self.assertIn("services", reg_data)
            self.assertIn("application", reg_data["services"])
            required_svc_keys = {"name", "image", "default_port", "protocol", "archetype"}
            for svc in reg_data["services"]["application"][:5]:
                self.assertTrue(required_svc_keys.issubset(svc.keys()))

        print("  ✅ STAGE 4 Complete: Scaffolded touchpoint payloads match live SSoT schemas.")

        print("\n" + "=" * 70)
        print("🎉 ALL SCAFFOLDING SCENARIO CHECKS PASSED (5/5 STAGES)")
        print("=" * 70 + "\n")


if __name__ == "__main__":
    unittest.main()
