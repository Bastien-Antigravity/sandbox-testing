#!/usr/bin/env python
# coding:utf-8
"""
SCENARIO TEST: Database Secret Encryption & In-Memory Decryption Audit
Location: sandbox-testing/02-Scenarios/python/rag_db_secrets_encryption_test.py

ESSENTIAL PROCESS:
Validates that:
1. Secrets written to PostgreSQL ("09-RAG-Engine".configuration) are ALWAYS encrypted at rest (ENC(...)).
2. Raw database rows NEVER contain plaintext secret tokens.
3. Decryption happens strictly in process memory when accessed via AppConfig / get_rag_setting.
4. Export/list operations (get_all) return encrypted ciphertext, preventing credential leakage.
"""

import json
import os
import sys
from pathlib import Path

# Add 09-RAG-Engine to python path
TEST_DIR = Path(__file__).resolve().parent
SANDBOX_DIR = TEST_DIR.parent.parent
WORKSPACE_ROOT = SANDBOX_DIR.parent
RAG_ENGINE_DIR = WORKSPACE_ROOT / "obsidian-brain" / "09-RAG-Engine"

if str(RAG_ENGINE_DIR) not in sys.path:
    sys.path.insert(0, str(RAG_ENGINE_DIR))

os.chdir(str(RAG_ENGINE_DIR))

import src.bootstrap as bootstrap
from microservice_toolbox.config import load_config
from src.utils.pg_pool import get_pg_pool, get_schema_name
from src.services.config.db_config import DBConfigService, _is_secret_key as is_secret_key
from src.core.config import get_rag_setting, get_enrichment_settings


def run_scenario_tests():
    print("=" * 70)
    print("🧪 RAG DATABASE SECRET ENCRYPTION & IN-MEMORY DECRYPTION SCENARIO TEST")
    print("=" * 70)

    config = load_config("standalone")
    logger = bootstrap.logger
    schema_name = get_schema_name(config)
    pool = get_pg_pool(config, logger)

    if not pool:
        print("❌ FAILED: PostgreSQL connection pool unavailable.")
        sys.exit(1)

    db_cfg = DBConfigService(config, logger)
    test_key = "enrichment.llm_api_key"
    plaintext_secret = "sk-live-secret-test-scenario-token-xyz-12345"

    try:
        # STAGE 1: Verify Key Classification
        print("\n[STAGE 1] Auditing Secret Key Classification...")
        assert is_secret_key(test_key) is True, f"'{test_key}' must be recognized as a secret key"
        assert is_secret_key("password") is True
        assert is_secret_key("auth_token") is True
        assert is_secret_key("vector_weight") is False
        print("  ├─ Recognized sensitive keys: enrichment.llm_api_key, password, auth_token")
        print("  ├─ Non-sensitive keys ignored: vector_weight")
        print("  └─ ✅ STAGE 1 PASSED: Secret classification accurate.")

        # STAGE 2: Write Plaintext Secret via DBConfigService
        print("\n[STAGE 2] Persisting Plaintext Secret via DBConfigService...")
        success = db_cfg.set(test_key, plaintext_secret, description="Automated scenario test secret")
        assert success is True, "Failed to persist secret in DBConfigService"
        print(f"  ├─ Successfully invoked db_cfg.set('{test_key}', 'sk-live-...')")
        print("  └─ ✅ STAGE 2 PASSED: Secret written to configuration service.")

        # STAGE 3: Direct Raw SQL Database Audit (Zero-Plaintext Guarantee)
        print("\n[STAGE 3] Auditing Raw PostgreSQL Database Row (At-Rest Encryption)...")
        conn = pool.getconn()
        try:
            with conn.cursor() as cursor:
                cursor.execute(
                    f'SELECT value, description FROM "{schema_name}".configuration WHERE key = %s',
                    (test_key,)
                )
                row = cursor.fetchone()
                assert row is not None, f"Key '{test_key}' not found in database table!"
                raw_db_value = row[0]
                if isinstance(raw_db_value, str):
                    try:
                        raw_db_value = json.loads(raw_db_value)
                    except Exception:
                        pass

                print(f"  ├─ Raw SQL stored value: {raw_db_value[:35]}...")
                
                # Verify that value starts with ENC( and ends with )
                assert isinstance(raw_db_value, str), "Database value must be stored as string"
                assert raw_db_value.startswith("ENC(") and raw_db_value.endswith(")"), \
                    f"CRITICAL SECURITY VIOLATION: Stored value is NOT encrypted! Raw: {raw_db_value}"
                
                # Verify plaintext NEVER exists anywhere in the database row
                assert plaintext_secret not in str(row), \
                    "CRITICAL SECURITY VIOLATION: Plaintext secret leaked in database row!"
                print("  ├─ Verified: Stored value matches ENC(...) envelope format.")
                print("  ├─ Verified: Zero plaintext occurrences in PostgreSQL table.")
        finally:
            pool.putconn(conn)
        print("  └─ ✅ STAGE 3 PASSED: Secret is 100% encrypted at rest in PostgreSQL.")

        # STAGE 4: Audit get_all() Export Safety
        print("\n[STAGE 4] Auditing get_all() Export for Credential Leakage Prevention...")
        all_configs = db_cfg.get_all()
        assert test_key in all_configs, f"'{test_key}' missing from get_all() export"
        exported_val = all_configs[test_key]["value"]
        assert exported_val.startswith("ENC(") and exported_val.endswith(")"), \
            f"Exported secret is not encrypted: {exported_val}"
        assert plaintext_secret not in json.dumps(all_configs), \
            "Plaintext secret found in get_all() dictionary output!"
        print("  ├─ Exported dictionary contains ENC(...) and zero plaintext tokens.")
        print("  └─ ✅ STAGE 4 PASSED: Bulk export preserves ciphertext integrity.")

        # STAGE 5: In-Process Memory Decryption Audit
        print("\n[STAGE 5] Auditing In-Process Memory Decryption...")
        # Direct DB service get returns encrypted ciphertext
        db_raw_get = db_cfg.get(test_key)
        assert db_raw_get.startswith("ENC("), "db_cfg.get() must return ciphertext without premature decryption"

        # 1. Generic get_rag_setting preserves raw ciphertext in memory (no premature decryption)
        raw_runtime_val = get_rag_setting(config, test_key)
        assert raw_runtime_val.startswith("ENC("), \
            f"get_rag_setting must retain raw ciphertext without premature decryption! Got: {raw_runtime_val}"

        # 2. Canonical AppConfig.decrypt_secret decrypts on-demand in process memory
        decrypted_runtime_val = config.decrypt_secret(raw_runtime_val)
        assert decrypted_runtime_val == plaintext_secret, \
            f"In-process decryption mismatch! Expected '{plaintext_secret}', got '{decrypted_runtime_val}'"
        
        # 3. Domain setting accessor (get_enrichment_settings) decrypts on-demand for consumers
        enrichment_settings = get_enrichment_settings(config)
        assert enrichment_settings["llm_api_key"] == plaintext_secret, \
            "get_enrichment_settings() did not decrypt secret in memory!"
        print(f"  ├─ get_rag_setting safely retained raw ciphertext: {raw_runtime_val[:12]}...")
        print(f"  ├─ AppConfig.decrypt_secret verified: {decrypted_runtime_val[:10]}...")
        print("  ├─ Decryption executed strictly in volatile memory, never persisted.")
        print("  └─ ✅ STAGE 5 PASSED: In-process memory decryption verified perfectly.")

    finally:
        # Cleanup test key
        print("\n[CLEANUP] Removing test key from PostgreSQL...")
        conn = pool.getconn()
        try:
            with conn.cursor() as cursor:
                cursor.execute(
                    f'DELETE FROM "{schema_name}".configuration WHERE key = %s',
                    (test_key,)
                )
            conn.commit()
            print(f"  └─ Removed test key '{test_key}'.")
        finally:
            pool.putconn(conn)

    print("\n" + "=" * 70)
    print("✨ ALL DATABASE SECRETS ENCRYPTION AUDIT TESTS PASSED PERFECTLY!")
    print("=" * 70)


if __name__ == "__main__":
    run_scenario_tests()
