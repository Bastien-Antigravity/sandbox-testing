package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Bastien-Antigravity/microservice-toolbox/go/pkg/business"
	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	mode := flag.String("mode", "ta", "Mode of operation: 'ta', 'ob', or 'ws'")
	natsURL := flag.String("nats", nats.DefaultURL, "NATS server URL")
	symbol := flag.String("symbol", "BTCUSDT,ETHUSDT", "Comma-separated symbols to simulate")
	port := flag.Int("port", 8080, "WebSocket port")
	flag.Parse()

	symbols := strings.Split(*symbol, ",")

	if *mode == "ws" {
		startWebSocketServer(*port, symbols)
		return
	}

	nc, err := nats.Connect(*natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()

	fmt.Printf("Mock Provider started in %s mode for %s\n", *mode, *symbol)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var counter int
	basePrice := 65000.0

	for {
		select {
		case <-ticker.C:
			counter++
			price := basePrice + float64(counter%100)*0.1

			for _, sym := range symbols {
				var event business.MarketEvent
				event.Symbol = sym
				event.Exchange = "MockExchange"
				event.Timestamp = uint64(time.Now().UnixMilli())
				event.EventID = fmt.Sprintf("evt-%d", time.Now().UnixNano())

				var subject string

				if *mode == "ta" {
					trade := business.Trade{
						Price:     price,
						Size:      1.0,
						Aggressor: business.AggressorBuy,
						TradeID:   fmt.Sprintf("t-%d", counter),
					}
					payload, _ := json.Marshal(trade)
					event.Type = business.TypeTrade
					event.Payload = payload
					subject = fmt.Sprintf("ohlcv.raw.%s", sym)
				} else {
					ob := business.OrderBook{
						Bids: []business.OrderBookLevel{
							{Price: price - 0.5, Size: 10.0},
							{Price: price - 1.0, Size: 20.0},
						},
						Asks: []business.OrderBookLevel{
							{Price: price + 0.5, Size: 10.0},
							{Price: price + 1.0, Size: 20.0},
						},
					}
					payload, _ := json.Marshal(ob)
					event.Type = business.TypeOrderBookSnapshot
					event.Payload = payload
					subject = fmt.Sprintf("marketdata.ORDERBOOK.%s", sym)
				}

				eventJSON, _ := json.Marshal(event)
				if err := nc.Publish(subject, eventJSON); err != nil {
					log.Printf("Failed to publish to NATS: %v", err)
				} else {
					log.Printf("[%s] Published %s event to %s at price %.2f", *mode, event.Type, subject, price)
				}
			}
		}
	}
}

func startWebSocketServer(port int, symbols []string) {
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WS upgrade error: %v", err)
			return
		}
		defer conn.Close()

		log.Printf("Client connected to WS for %v", symbols)

		ticker := time.NewTicker(500 * time.Millisecond) // Faster for multiple symbols
		defer ticker.Stop()

		var counter int
		basePrice := 65000.0

		for {
			select {
			case <-ticker.C:
				counter++
				
				for _, sym := range symbols {
					price := fmt.Sprintf("%.2f", basePrice+float64(counter%100)*0.1)

					// Send Binance-like trade event
					trade := map[string]interface{}{
						"e": "trade",
						"E": time.Now().UnixMilli(),
						"s": sym,
						"t": counter,
						"p": price,
						"q": "1.0",
						"T": time.Now().UnixMilli(),
						"m": true,
					}
					data, _ := json.Marshal(trade)
					if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
						log.Printf("WS write error: %v", err)
						return
					}

					// Send Binance-like depthUpdate event
					depth := map[string]interface{}{
						"e": "depthUpdate",
						"E": time.Now().UnixMilli(),
						"s": sym,
						"U": int64(counter),
						"u": int64(counter + 2),
						"b": [][]string{
							{fmt.Sprintf("%.2f", basePrice+float64(counter%100)*0.1-0.5), "10.0"},
							{fmt.Sprintf("%.2f", basePrice+float64(counter%100)*0.1-1.0), "20.0"},
						},
						"a": [][]string{
							{fmt.Sprintf("%.2f", basePrice+float64(counter%100)*0.1+0.5), "10.0"},
							{fmt.Sprintf("%.2f", basePrice+float64(counter%100)*0.1+1.0), "20.0"},
						},
					}
					depthData, _ := json.Marshal(depth)
					if err := conn.WriteMessage(websocket.TextMessage, depthData); err != nil {
						log.Printf("WS write error: %v", err)
						return
					}
				}
				log.Printf("[WS] Sent trades and depth updates for %d symbols", len(symbols))
			}
		}
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Mock WebSocket server starting on %s for %v\n", addr, symbols)
	log.Fatal(http.ListenAndServe(addr, nil))
}
