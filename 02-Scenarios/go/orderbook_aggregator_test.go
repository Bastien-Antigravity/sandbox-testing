package scenarios

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
)

func TestOrderbookAggregation(t *testing.T) {
	t.Run("Aggregation_Publishing_Verification", func(t *testing.T) {
		fmt.Println(">>> Scenario Test: Verifying Orderbook Aggregation Publishing")
		
		// Connect to NATS
		nc, err := nats.Connect(nats.DefaultURL)
		assert.NoError(t, err)
		defer nc.Close()

		// Channel to receive aggregated data
		dataChan := make(chan []byte, 10)
		
		// Subscribe to the aggregator's output
		// Based on default config: prefix=orderbook, type=OrderBook, symbol=BTCUSDT
		sub, err := nc.Subscribe("orderbook.OrderBook.BTCUSDT", func(msg *nats.Msg) {
			dataChan <- msg.Data
		})
		assert.NoError(t, err)
		defer sub.Unsubscribe()

		fmt.Println(">>> Waiting for aggregated orderbook data on NATS...")
		
		select {
		case data := <-dataChan:
			fmt.Println(">>> Received aggregated data!")
			
			var result map[string]interface{}
			err := json.Unmarshal(data, &result)
			assert.NoError(t, err)
			
			assert.Equal(t, "BTCUSDT", result["symbol"])
			assert.Equal(t, "OrderBook", result["data_type"])
			
			// Verify we have bids and asks
			if bids, ok := result["bids"].([]interface{}); ok {
				fmt.Printf(">>> Aggregated Bids: %d levels\n", len(bids))
				assert.Greater(t, len(bids), 0)
			} else {
				t.Errorf("Missing or invalid bids in aggregated data")
			}
			
		case <-time.After(15 * time.Second):
			t.Errorf("Timed out waiting for aggregated data on NATS")
		}
	})
}
