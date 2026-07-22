package scenarios

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
)

func TestTriplePipeline(t *testing.T) {
	// 1. Verify Aggregator Output on NATS
	t.Run("NATS_Aggregated_OrderBook_Publishing", func(t *testing.T) {
		fmt.Println(">>> [TestTriplePipeline] Verifying Aggregated OrderBook on NATS...")
		nc, err := nats.Connect(nats.DefaultURL)
		assert.NoError(t, err)
		defer nc.Close()

		dataChan := make(chan []byte, 10)
		sub, err := nc.Subscribe("orderbook.OrderBook.BTCUSDT", func(msg *nats.Msg) {
			dataChan <- msg.Data
		})
		assert.NoError(t, err)
		defer sub.Unsubscribe()

		select {
		case data := <-dataChan:
			fmt.Println(">>> [TestTriplePipeline] Received aggregated order book data from NATS!")
			var result map[string]interface{}
			err := json.Unmarshal(data, &result)
			assert.NoError(t, err)
			assert.Equal(t, "BTCUSDT", result["symbol"])
			assert.Equal(t, "OrderBook", result["data_type"])

			bids, bidsOk := result["bids"].([]interface{})
			asks, asksOk := result["asks"].([]interface{})
			assert.True(t, bidsOk && asksOk)
			assert.Greater(t, len(bids), 0)
			assert.Greater(t, len(asks), 0)

		case <-time.After(20 * time.Second):
			t.Errorf("Timed out waiting for aggregated orderbook data on NATS topic: orderbook.OrderBook.BTCUSDT")
		}
	})

	// 2. Verify Technical Analysis persistence in TimescaleDB (Postgres)
	t.Run("Postgres_Technical_Analysis_Persistence", func(t *testing.T) {
		fmt.Println(">>> [TestTriplePipeline] Verifying persisted indicators in TimescaleDB...")
		connStr := os.Getenv("TS_CONN_STR")
		if connStr == "" {
			connStr = "postgresql://dbuser:dbuser@127.0.0.1:5432/maindb?sslmode=disable"
		}
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			t.Fatalf("Failed to connect to database: %v", err)
		}
		defer db.Close()

		// Wait for table creation and data insertion (up to 20 retries)
		var schemaName string
		query := `
			SELECT table_schema 
			FROM information_schema.tables 
			WHERE table_name = 'btcusdt_ohlcv' 
			LIMIT 1
		`
		for i := 0; i < 20; i++ {
			err = db.QueryRow(query).Scan(&schemaName)
			if err == nil && schemaName != "" {
				break
			}
			time.Sleep(2 * time.Second)
		}
		assert.NoError(t, err, "Table btcusdt_ohlcv not found in database schemas")
		fmt.Printf(">>> [TestTriplePipeline] Found technical-analysis schema: %s\n", schemaName)

		// Check OHLCV table entries
		var ohlcvCount int
		err = db.QueryRow(fmt.Sprintf(`SELECT count(*) FROM "%s"."btcusdt_ohlcv"`, schemaName)).Scan(&ohlcvCount)
		assert.NoError(t, err)
		fmt.Printf(">>> [TestTriplePipeline] BTCUSDT OHLCV entry count: %d\n", ohlcvCount)
		assert.Greater(t, ohlcvCount, 0, "Expected at least one OHLCV record persisted")

		// Check indicators table entries (from technical-analysis calculation)
		var taCount int
		err = db.QueryRow(fmt.Sprintf(`SELECT count(*) FROM "%s"."btcusdt_ta"`, schemaName)).Scan(&taCount)
		assert.NoError(t, err)
		fmt.Printf(">>> [TestTriplePipeline] BTCUSDT TA Indicators entry count: %d\n", taCount)
		assert.Greater(t, taCount, 0, "Expected at least one technical indicator calculation record persisted")
	})
}
