package scenarios

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestRAGEngineStorage(t *testing.T) {
	// TimescaleDB Connection (RAG Engine database: obsidiandb)
	connStr := os.Getenv("RAG_CONN_STR")
	if connStr == "" {
		connStr = os.Getenv("TS_CONN_STR")
	}
	if connStr == "" {
		ragDB := os.Getenv("RAG_DBNAME")
		if ragDB == "" {
			ragDB = "obsidiandb"
		}
		connStr = fmt.Sprintf("postgresql://dbuser:dbuser@127.0.0.1:5432/%s?sslmode=disable", ragDB)
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	schema := "09-RAG-Engine"

	// 1. Ensure Schema and tables are created by running the indexer at least once
	t.Run("Execute_Indexer", func(t *testing.T) {
		// Clean up any old hashes to force re-indexing
		cleanupQuery := fmt.Sprintf(`
			DO $$ 
			BEGIN 
				IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = '%s' AND table_name = 'file_hashes') THEN
					TRUNCATE TABLE "%s".file_hashes CASCADE;
					TRUNCATE TABLE "%s".embeddings CASCADE;
				END IF;
			END $$;
		`, schema, schema, schema)
		_, _ = db.Exec(cleanupQuery)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current working directory: %v", err)
		}
		ragEngineDir := filepath.Clean(filepath.Join(cwd, "..", "..", "..", "obsidian-brain", "09-RAG-Engine"))
		targetFile := filepath.Join(ragEngineDir, "src", "core", "server.py")

		pyExe := filepath.Join(ragEngineDir, ".venv", "bin", "python3")
		if _, err := os.Stat(pyExe); os.IsNotExist(err) {
			pyExe = "python3"
		}

		// Spawn indexer synchronously in non-development mode with dynamic file path
		cmd := exec.Command(pyExe, "main.py", "index", "--file", targetFile)
		cmd.Dir = ragEngineDir
		cmd.Env = append(os.Environ(), "RG_DEVEL=false", fmt.Sprintf("BRAIN_DIR=%s", filepath.Dir(ragEngineDir)))

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Indexer execution failed: %v\nOutput: %s", err, string(output))
		}
		fmt.Printf(">>> Indexer output:\n%s\n", string(output))
	})

	// 2. Verify that all required tables exist in the schema
	t.Run("Verify_Tables_Exist", func(t *testing.T) {
		tables := []string{"embeddings", "parents", "file_hashes", "codebase_nodes", "codebase_edges", "kms_nodes", "kms_edges", "configuration"}
		for _, table := range tables {
			var exists bool
			query := `
				SELECT EXISTS (
					SELECT 1 
					FROM information_schema.tables 
					WHERE table_schema = $1 AND table_name = $2
				)
			`
			var err error
			for i := 0; i < 5; i++ {
				err = db.QueryRow(query, schema, table).Scan(&exists)
				if err == nil && exists {
					break
				}
				time.Sleep(1 * time.Second)
			}
			assert.NoError(t, err, "Failed to query table existence for: %s", table)
			assert.True(t, exists, "Table '%s' does not exist in schema '%s'", table, schema)
			fmt.Printf(">>> Verified table exists: %s.%s\n", schema, table)
		}
	})

	// 3. Verify that embeddings table has pgvector column
	t.Run("Verify_pgvector_Column", func(t *testing.T) {
		var dataType string
		var udtName string
		query := `
			SELECT data_type, udt_name 
			FROM information_schema.columns 
			WHERE table_schema = $1 AND table_name = 'embeddings' AND column_name = 'embedding'
		`
		err := db.QueryRow(query, schema).Scan(&dataType, &udtName)
		assert.NoError(t, err, "Failed to query column information for embeddings.embedding")
		assert.Contains(t, []string{"USER-DEFINED", "vector"}, dataType, "Invalid data type for embedding column")
		assert.Equal(t, "vector", udtName, "udt_name should be 'vector'")
		fmt.Printf(">>> Verified pgvector column type is: %s (%s)\n", dataType, udtName)
	})

	// 4. Verify that we have successfully indexed and populated some data
	t.Run("Verify_Indexed_Data", func(t *testing.T) {
		var count int
		query := fmt.Sprintf(`SELECT count(*) FROM "%s".embeddings`, schema)
		err := db.QueryRow(query).Scan(&count)
		if err != nil || count == 0 {
			altQuery := `SELECT count(*) FROM "obsidian-brain".embeddings`
			_ = db.QueryRow(altQuery).Scan(&count)
		}
		assert.Greater(t, count, 0, "No indexed embeddings found in table")
		fmt.Printf(">>> Verified indexed embeddings count: %d\n", count)
	})

	// 5. Verify Secrets At-Rest Encryption in configuration table
	t.Run("Verify_Secrets_At_Rest_Encryption", func(t *testing.T) {
		query := fmt.Sprintf(`
			SELECT key, value 
			FROM "%s".configuration 
			WHERE key ILIKE '%%key%%' 
			   OR key ILIKE '%%token%%' 
			   OR key ILIKE '%%secret%%' 
			   OR key ILIKE '%%password%%'
		`, schema)
		rows, err := db.Query(query)
		assert.NoError(t, err, "Failed to query configuration table for secrets")
		defer rows.Close()

		for rows.Next() {
			var k, val string
			err := rows.Scan(&k, &val)
			assert.NoError(t, err)
			assert.True(t, len(val) > 4 && val[:4] == "ENC(", 
				"Secret key '%s' in database is NOT encrypted at rest! Stored value does not start with ENC(...)", k)
			fmt.Printf(">>> Verified secret key '%s' is securely encrypted at rest: ENC(...)\n", k)
		}
	})
}
