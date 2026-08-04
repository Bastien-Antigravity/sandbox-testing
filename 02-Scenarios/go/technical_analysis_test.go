package scenarios

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	_ "modernc.org/sqlite"
)

func TestTechnicalAnalysis(t *testing.T) {
	t.Run("Indicator_Calculation_Verification", func(t *testing.T) {
		fmt.Println(">>> Scenario Test: Verifying Technical Analysis Indicators")

		// Path to the SQLite database created by the technical-analysis service
		// The orchestrator runs the service in its own directory
		dbPath := "../../../technical-analysis/technical-analysis.db"

		var db *sql.DB
		var err error

		// Retry connecting to DB as it might take a moment to be created
		for i := 0; i < 10; i++ {
			db, err = sql.Open("sqlite", dbPath)
			if err == nil {
				err = db.Ping()
			}
			if err == nil {
				break
			}
			fmt.Printf(">>> Waiting for database... (%d/10)\n", i+1)
			time.Sleep(2 * time.Second)
		}
		assert.NoError(t, err, "Failed to connect to SQLite database")
		defer db.Close()

		// Check if OHLCV data is being populated
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM BTCUSDT_ohlcv").Scan(&count)
		assert.NoError(t, err)
		fmt.Printf(">>> Found %d OHLCV entries\n", count)
		assert.Greater(t, count, 0, "No OHLCV data found in database")

		// Check if some indicators are being calculated (e.g. SMA)
		// We'll check the SMA table (assuming SMA is configured)
		var smaCount int
		// We use a dynamic query because indicator names might vary
		rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'BTCUSDT_%'")
		assert.NoError(t, err)

		foundIndicator := false
		for rows.Next() {
			var tableName string
			rows.Scan(&tableName)
			if tableName != "BTCUSDT_ohlcv" {
				fmt.Printf(">>> Found indicator table: %s\n", tableName)
				foundIndicator = true

				err = db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&smaCount)
				assert.NoError(t, err)
				fmt.Printf(">>> Found %d entries in %s\n", smaCount, tableName)
			}
		}
		rows.Close()

		assert.True(t, foundIndicator, "No indicator tables found (other than OHLCV)")
	})
}
