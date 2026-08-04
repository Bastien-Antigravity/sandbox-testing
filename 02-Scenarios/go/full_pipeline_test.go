package scenarios

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestFullPipeline(t *testing.T) {
	// TimescaleDB Connection (default for local sandbox, can be overridden by TS_CONN_STR)
	connStr := os.Getenv("TS_CONN_STR")
	if connStr == "" {
		connStr = "postgresql://dbuser:dbuser@127.0.0.1:5432/maindb?sslmode=disable"
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	symbols := []string{"BTCUSDT", "ETHUSDT"}

	t.Run("Technical_Analysis_Indicators", func(t *testing.T) {
		fmt.Println(">>> Verifying Technical Indicators in Technical-Analysis Schema")

		// Technical Analysis usually uses its service name as schema, often 'public' or 'technical_analysis'
		// We'll search for the 'btcusdt_ohlcv' table
		var schemaName string
		query := `
			SELECT table_schema 
			FROM information_schema.tables 
			WHERE table_name = 'btcusdt_ohlcv' 
			LIMIT 1
		`
		var err error
		for i := 0; i < 15; i++ {
			err = db.QueryRow(query).Scan(&schemaName)
			if err == nil && schemaName != "" {
				break
			}
			time.Sleep(2 * time.Second)
		}
		assert.NoError(t, err, "Table BTCUSDT_ohlcv not found in any schema")
		fmt.Printf(">>> Found technical-analysis schema: %s\n", schemaName)

		for _, sym := range symbols {
			cleanSym := strings.ToLower(sym) // sanitize if needed, but btcusdt/ethusdt are safe
			ohlcvTable := fmt.Sprintf("%s_ohlcv", cleanSym)
			taTable := fmt.Sprintf("%s_ta", cleanSym)

			var ohlcvCount int
			err = db.QueryRow(fmt.Sprintf(`SELECT count(*) FROM "%s"."%s"`, schemaName, ohlcvTable)).Scan(&ohlcvCount)
			assert.NoError(t, err)
			fmt.Printf(">>> [%s] OHLCV count: %d\n", sym, ohlcvCount)
			assert.Greater(t, ohlcvCount, 0)

			var taCount int
			err = db.QueryRow(fmt.Sprintf(`SELECT count(*) FROM "%s"."%s"`, schemaName, taTable)).Scan(&taCount)
			assert.NoError(t, err)
			fmt.Printf(">>> [%s] Indicators count: %d\n", sym, taCount)
			assert.Greater(t, taCount, 0)
		}
	})
}
