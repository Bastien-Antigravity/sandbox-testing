package scenarios

import (
	"fmt"
	"testing"

	"github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/business"
	"github.com/stretchr/testify/assert"
)

func TestUnifiedDataStandard(t *testing.T) {
	fmt.Println(">>> Sandbox Scenario: FEAT-004-Unified-Data")

	t.Run("MarketEvent_L1_Integration", func(t *testing.T) {
		fmt.Println(">>> Step 1: MarketEvent (Trade) Serialization/Deserialization")
		
		trade := business.Trade{
			Price:     45000.75,
			Size:      0.05,
			Aggressor: business.AggressorSell,
			TradeID:   "TR-998877",
		}

		// Wrap in envelope
		event, err := business.WrapMarketEvent("BTC/USDT", "Coinbase", business.TypeTrade, trade)
		assert.NoError(t, err)
		assert.Equal(t, "BTC/USDT", event.Symbol)

		// Mock network transmission (Serialize)
		wireData, err := business.Serialize(event)
		assert.NoError(t, err)

		// Mock consumer (Deserialize)
		var receivedEvent business.MarketEvent
		err = business.Deserialize(wireData, &receivedEvent)
		assert.NoError(t, err)
		assert.Equal(t, business.TypeTrade, receivedEvent.Type)

		// Unwrap payload
		var receivedTrade business.Trade
		err = business.Deserialize(receivedEvent.Payload, &receivedTrade)
		assert.NoError(t, err)
		assert.Equal(t, 45000.75, receivedTrade.Price)
		assert.Equal(t, business.AggressorSell, receivedTrade.Aggressor)
	})

	t.Run("OHLCV_Bar_Integrity", func(t *testing.T) {
		fmt.Println(">>> Step 2: OHLCV Bar Integrity")
		
		bar := business.OHLCV{
			Symbol:   "ETH/USDT",
			Interval: "5m",
			Open:     3200.0,
			High:     3215.5,
			Low:      3190.0,
			Close:    3210.2,
			Volume:   1500.0,
			VWAP:     3205.8,
			Trades:   450,
		}

		wireData, err := business.Serialize(bar)
		assert.NoError(t, err)

		var receivedBar business.OHLCV
		err = business.Deserialize(wireData, &receivedBar)
		assert.NoError(t, err)
		assert.Equal(t, "ETH/USDT", receivedBar.Symbol)
		assert.Equal(t, 3215.5, receivedBar.High)
		assert.Equal(t, uint32(450), receivedBar.Trades)
	})

	t.Run("Strategy_Signal_Precision", func(t *testing.T) {
		fmt.Println(">>> Step 3: Strategy Signal Precision")
		
		signal := business.Signal{
			Source:   "alpha-v1",
			Symbol:   "SOL/USDT",
			Type:     business.SignalBuy,
			Strength: 0.92,
			Price:    145.50,
			Metadata: `{"reason": "RSI Oversold", "conf": 0.95}`,
		}

		wireData, err := business.Serialize(signal)
		assert.NoError(t, err)

		var receivedSignal business.Signal
		err = business.Deserialize(wireData, &receivedSignal)
		assert.NoError(t, err)
		assert.Equal(t, business.SignalBuy, receivedSignal.Type)
		assert.Equal(t, float32(0.92), receivedSignal.Strength)
		assert.Contains(t, receivedSignal.Metadata, "RSI Oversold")
	})
}
