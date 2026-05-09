package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type BinanceTrade struct {
	Event     string `json:"e"`
	Time      int64  `json:"E"`
	Symbol    string `json:"s"`
	TradeID   int64  `json:"t"`
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	BuyerID   int64  `json:"b"`
	SellerID  int64  `json:"a"`
	TradeTime int64  `json:"T"`
	IsBuyer   bool   `json:"m"`
	Ignore    bool   `json:"M"`
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Println("Client connected")

	// Read subscription message
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Printf("Read error: %v", err)
		return
	}
	log.Printf("Received: %s", string(msg))

	// Send mock trades every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			trade := BinanceTrade{
				Event:     "trade",
				Time:      time.Now().UnixMilli(),
				Symbol:    "BTCUSDT",
				TradeID:   12345,
				Price:     "65000.00",
				Quantity:  "0.01",
				TradeTime: time.Now().UnixMilli(),
				IsBuyer:   true,
			}
			data, _ := json.Marshal(trade)
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("Write error: %v", err)
				return
			}
			log.Printf("Sent mock trade: %s", trade.Price)
		}
	}
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)
	fmt.Println("Mock Binance WS server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
