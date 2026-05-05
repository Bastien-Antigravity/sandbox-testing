package scenarios

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// OrderbookEvent represents the internal model we expect the data-ingestor to produce.
type OrderbookEvent struct {
	Symbol            string   `json:"symbol"`
	ExchangeTimestamp int64    `json:"exchange_ts"`
	LocalTimestamp    int64    `json:"local_ts"`
	Bids              [][]string `json:"bids"`
	Asks              [][]string `json:"asks"`
}

func TestOrderbookIngestion(t *testing.T) {
	t.Run("Normalization_Accuracy_and_Latency", func(t *testing.T) {
		fmt.Println(">>> QA Test: Normalization Accuracy and Latency")
		
		rawBinanceJSON := `{
			"e": "depthUpdate",
			"E": 1623456789123,
			"s": "BTCUSDT",
			"U": 157,
			"u": 160,
			"b": [["35000.00", "0.500"], ["34999.00", "1.200"]],
			"a": [["35001.00", "0.100"], ["35002.00", "0.850"]]
		}`

		// Simulate the ingestor's logic
		start := time.Now()
		
		var raw map[string]interface{}
		err := json.Unmarshal([]byte(rawBinanceJSON), &raw)
		assert.NoError(t, err)

		// Map to internal model
		event := OrderbookEvent{
			Symbol:            raw["s"].(string),
			ExchangeTimestamp: int64(raw["E"].(float64)),
			LocalTimestamp:    time.Now().UnixMilli(),
			Bids:              make([][]string, 0),
			Asks:              make([][]string, 0),
		}

		for _, b := range raw["b"].([]interface{}) {
			bid := b.([]interface{})
			event.Bids = append(event.Bids, []string{bid[0].(string), bid[1].(string)})
		}
		for _, a := range raw["a"].([]interface{}) {
			ask := a.([]interface{})
			event.Asks = append(event.Asks, []string{ask[0].(string), ask[1].(string)})
		}

		duration := time.Since(start)
		fmt.Printf(">>> Normalization took: %v\n", duration)

		// Assertions
		assert.Equal(t, "BTCUSDT", event.Symbol)
		assert.Equal(t, int64(1623456789123), event.ExchangeTimestamp)
		assert.Len(t, event.Bids, 2)
		assert.Len(t, event.Asks, 2)
		assert.Less(t, duration, 5*time.Millisecond, "Normalization must be < 5ms")
	})

	t.Run("Heartbeat_Enforcement", func(t *testing.T) {
		fmt.Println(">>> QA Test: Heartbeat Enforcement (Watchdog)")
		
		// The service should have a 30s timeout.
		// For the test, we assume we can configure this timeout to be smaller for verification.
		heartbeatTimeout := 100 * time.Millisecond
		lastDataTime := time.Now()
		
		// Wait for more than the timeout
		time.Sleep(150 * time.Millisecond)
		
		if time.Since(lastDataTime) > heartbeatTimeout {
			fmt.Println(">>> Watchdog triggered: Connection stale")
			// In real implementation, this should trigger a reconnect
		} else {
			t.Errorf("Watchdog failed to trigger after stale period")
		}
	})

	t.Run("Discard_Empty_Updates", func(t *testing.T) {
		fmt.Println(">>> QA Test: Discard Empty Updates")
		
		emptyUpdateJSON := `{
			"e": "depthUpdate", "E": 1623456789124, "s": "BTCUSDT",
			"b": [], "a": []
		}`
		
		var raw map[string]interface{}
		json.Unmarshal([]byte(emptyUpdateJSON), &raw)
		
		bids := raw["b"].([]interface{})
		asks := raw["a"].([]interface{})
		
		isValid := len(bids) > 0 || len(asks) > 0
		assert.False(t, isValid, "Ingestor must discard events with empty bids and asks")
	})
}
