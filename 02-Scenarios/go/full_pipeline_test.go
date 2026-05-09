package scenarios

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

func TestFullPipeline(t *testing.T) {
	connStr := "postgresql://dbuser:dbuser@127.0.0.1:5432/maindb?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Wait for data to arrive (retry loop)
	symbol := "BTCUSDT"
	var count int
	
	fmt.Println(">>> Waiting for data in TimescaleDB...")
	
	for i := 0; i < 30; i++ {
		// We search in all schemas to be resilient to different executable names
		query := `
			SELECT count(*) 
			FROM information_schema.tables 
			WHERE table_name = 'stock_prices_tick'
		`
		var tableExists int
		db.QueryRow(query).Scan(&tableExists)
		
		if tableExists > 0 {
			// Find the schema
			var schemaName string
			db.QueryRow("SELECT table_schema FROM information_schema.tables WHERE table_name = 'stock_prices_tick' LIMIT 1").Scan(&schemaName)
			
			row := db.QueryRow(fmt.Sprintf(`SELECT count(*) FROM "%s"."stock_prices_tick" WHERE symbol = $1`, schemaName), symbol)
			err = row.Scan(&count)
			if err == nil && count > 0 {
				fmt.Printf(">>> Found %d records for %s in schema %s\n", count, symbol, schemaName)
				break
			}
		}
		
		time.Sleep(1 * time.Second)
	}

	assert.Greater(t, count, 0, "No data found for BTCUSDT in TimescaleDB after 30 seconds")
}
