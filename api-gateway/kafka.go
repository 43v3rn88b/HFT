package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"
)

// NewKafkaReader creates a connection to the Redpanda broker
func NewKafkaReader(brokerURL string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{brokerURL}, // Same port we used for Rust!
		Topic:    "orderbook.aapl",
		GroupID:  "api-gateway-group",         // Unique ID for the Go API
		MaxBytes: 10e6,                        // 10MB
	})
}

// ConsumeSnapshots handles the background listening
func ConsumeSnapshots(manager *OrderBookManager, reader *kafka.Reader) {
	for {
		msg, err := reader.ReadMessage(context.Background()) 
		if err != nil {
			log.Printf("Kafka read error: %v", err) 
			continue
		}

		var snapshot OrderBookSnapshot 
		if err := json.Unmarshal(msg.Value, &snapshot); err != nil { 
			log.Printf("Failed to unmarshal snapshot: %v", err) 
			continue
		}

		// 🛡️ GUARD RAIL: Ignore empty or invalid tickers
        if snapshot.Ticker == "" || snapshot.Ticker == "." {
            log.Println("⚠️ Received malformed snapshot (empty ticker). Skipping...")
            continue
        }

		// Update the thread-safe state manager
		manager.Update(snapshot) 
		log.Printf("📥 Book updated for %s. Bids: %d, Asks: %d", 
			snapshot.Ticker, len(snapshot.Bids), len(snapshot.Asks))
	}
}