package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
)

type Trade struct {
	BuyOrderID  string `json:"buy_order_id"`
	SellOrderID string `json:"sell_order_id"`
	Ticker      string `json:"ticker"`
	Price       uint64 `json:"price"`
	Quantity    uint64 `json:"quantity"`
}

func main() {
	dbURL := os.Getenv("DB_URL")
	kafkaBroker := os.Getenv("KAFKA_BROKER")

	// 1. Connect to PostgreSQL with retries
	var db *sql.DB
	var err error
	for i := 0; i < 5; i++ {
		db, err = sql.Open("postgres", dbURL)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		log.Printf("⏳ Waiting for database... (Attempt %d/5)", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal("❌ Failed to connect to DB:", err)
	}
	defer db.Close()

	// 2. Configure Kafka Consumer
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{kafkaBroker},
		Topic:   "trades.aapl",
		GroupID: "settler-group",
	})
	defer reader.Close()

	fmt.Println("📉 Settler is online. Waiting for trades to persist...")

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Println("Error reading message:", err)
			continue
		}

		var trade Trade
		if err := json.Unmarshal(m.Value, &trade); err != nil {
			log.Println("Failed to parse trade JSON:", err)
			continue
		}

		// 3. Insert into PostgreSQL
		query := `INSERT INTO trades (buy_order_id, sell_order_id, ticker, price, quantity) VALUES ($1, $2, $3, $4, $5)`
		_, err = db.Exec(query, trade.BuyOrderID, trade.SellOrderID, trade.Ticker, trade.Price, trade.Quantity)
		
		if err != nil {
			log.Printf("❌ Failed to settle trade %s: %v", trade.BuyOrderID, err)
		} else {
			fmt.Printf("✅ Settled Trade: %d units of %s at $%d.%d\n", 
				trade.Quantity, trade.Ticker, trade.Price/100, trade.Price%100)
		}
	}
}