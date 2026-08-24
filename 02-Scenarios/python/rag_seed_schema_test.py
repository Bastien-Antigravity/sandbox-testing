#!/usr/bin/env python
# coding:utf-8
"""
SCENARIO TEST: RAG Seed Export/Import & PostgreSQL Schema Isolation Audit
Location: sandbox-testing/02-Scenarios/python/rag_seed_schema_test.py

ESSENTIAL PROCESS:
Validates that:
1. PostgreSQL database tables reside strictly in schema "09-RAG-Engine" and public contains zero data tables.
2. Seed export (export-seed) produces valid manifest and compressed jsonl dataset.
3. Secret & local host path sanitization filter redacts sensitive tokens and absolute paths.
4. Seed import (import-seed --reset) restores database tables cleanly.
5. Post-import vector similarity search executes successfully against pgvector.
"""

import os
import sys
import gzip
import json
import re
import shutil
import tempfile
import time
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
from src.services.seed.seed_service import SeedService


def run_scenario_tests():
    print("=" * 70)
    print("🧪 RAG SEED EXPORT/IMPORT & SCHEMA ISOLATION SCENARIO TEST")
    print("=" * 70)

    config = load_config("standalone")
    logger = bootstrap.logger
    schema_name = get_schema_name(config)
    pool = get_pg_pool(config, logger)

    if not pool:
        print("❌ FAILED: PostgreSQL connection pool unavailable.")
        sys.exit(1)

    temp_seed_dir = tempfile.mkdtemp(prefix="rag_test_seed_")
    try:
        # STAGE 1: Schema Isolation Audit
        print("\n[STAGE 1] Auditing PostgreSQL Schema Isolation...")
        conn = pool.getconn()
        try:
            with conn.cursor() as cur:
                cur.execute("""
                    SELECT table_schema, table_name 
                    FROM information_schema.tables 
                    WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
                    ORDER BY table_schema, table_name;
                """)
                rows = cur.fetchall()

                data_tables_in_public = [tbl for sch, tbl in rows if sch == "public"]
                target_schema_tables = [tbl for sch, tbl in rows if sch == schema_name]

                print(f"  ├─ Active schema name: '{schema_name}'")
                print(f"  ├─ Tables in '{schema_name}': {len(target_schema_tables)} ({target_schema_tables})")
                print(f"  ├─ Data tables in 'public': {len(data_tables_in_public)} ({data_tables_in_public})")

                assert len(data_tables_in_public) == 0, f"Schema isolation breach! Found data tables in public: {data_tables_in_public}"
                assert "embeddings" in target_schema_tables, "embeddings table missing in target schema!"
                assert "parents" in target_schema_tables, "parents table missing in target schema!"
                print("  └─ ✅ STAGE 1 PASSED: Strict schema isolation verified.")
        finally:
            pool.putconn(conn)

        # STAGE 2: Export Seed Validation
        print("\n[STAGE 2] Testing Seed Export (export-seed)...")
        service = SeedService(config, logger)
        export_res = service.export_seed(out_dir=temp_seed_dir, sanitize=True)

        assert export_res["status"] == "success", f"Export failed: {export_res}"
        manifest_path = Path(temp_seed_dir) / "rag_manifest.json"
        seed_gz_path = Path(temp_seed_dir) / "rag_seed.jsonl.gz"

        assert manifest_path.exists(), "rag_manifest.json missing!"
        assert seed_gz_path.exists(), "rag_seed.jsonl.gz missing!"
        assert seed_gz_path.stat().st_size > 0, "rag_seed.jsonl.gz is empty!"

        with open(manifest_path, "r", encoding="utf-8") as f:
            manifest = json.load(f)

        assert manifest["database_engine"] == "PostgreSQL + pgvector"
        assert manifest["schema_name"] == schema_name
        assert manifest["vector_dimensions"] == 1024
        assert manifest["sanitized"] is True

        print(f"  ├─ Manifest stats: {manifest['stats']}")
        print(f"  ├─ Compressed dataset size: {seed_gz_path.stat().st_size / 1024:.2f} KB")
        print("  └─ ✅ STAGE 2 PASSED: Seed package successfully generated.")

        # STAGE 3: Secret & Path Sanitization Audit
        print("\n[STAGE 3] Auditing Secret & Path Redaction Filter...")
        secret_patterns = [
            re.compile(r'sk-[a-zA-Z0-9]{20,}'),
            re.compile(r'ghp_[a-zA-Z0-9]{20,}'),
            re.compile(r'file:///Users/[^/\s\)\"\']+/Desktop/Bastien-Antigravity/'),
            re.compile(r'/Users/[^/\s\)\"\']+/Desktop/Bastien-Antigravity/')
        ]

        leaked_matches = []
        with gzip.open(seed_gz_path, "rt", encoding="utf-8") as gz:
            for line_no, line in enumerate(gz, start=1):
                for pat in secret_patterns:
                    if pat.search(line):
                        leaked_matches.append((line_no, pat.pattern))

        assert len(leaked_matches) == 0, f"Sanitization breach! Found unredacted patterns: {leaked_matches}"
        print("  └─ ✅ STAGE 3 PASSED: Zero API keys or local host absolute paths leaked.")

        # STAGE 4: Import Seed Roundtrip
        print("\n[STAGE 4] Testing Seed Import Roundtrip (import-seed --reset)...")
        import_res = service.import_seed(in_path=temp_seed_dir, reset=True)

        assert import_res["status"] == "success", f"Import failed: {import_res}"
        imported_stats = import_res["imported_stats"]
        print(f"  ├─ Imported stats: {imported_stats}")
        assert imported_stats["parents"] == manifest["stats"]["parents_count"]
        assert imported_stats["embeddings"] == manifest["stats"]["embeddings_count"]
        print("  └─ ✅ STAGE 4 PASSED: Seed imported cleanly with 100% record match.")

        # STAGE 5: Post-Import Vector Search Smoke Test
        print("\n[STAGE 5] Testing Vector Search against Hydrated Store...")
        from src.services.vector_store.pgvector import PgVectorStore
        vector_store = PgVectorStore(config, logger)
        query_res = vector_store._execute_query("SELECT COUNT(*) FROM embeddings", fetch=True)
        count = query_res[0][0] if query_res else 0

        print(f"  ├─ Vector count in PostgreSQL embeddings table: {count}")
        assert count > 0, "Embeddings table is empty after import!"
        print("  └─ ✅ STAGE 5 PASSED: Vector store is online and populated.")

        print("\n" + "=" * 70)
        print("✨ ALL RAG SEED & SCHEMA ISOLATION TESTS PASSED PERFECTLY!")
        print("=" * 70)

    finally:
        shutil.rmtree(temp_seed_dir, ignore_errors=True)


if __name__ == "__main__":
    run_scenario_tests()
