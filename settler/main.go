package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
	"strings"

	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	"github.com/redis/go-redis/v9"
)

const settleTradeScript = `
local buyer_hold_key = KEYS[1]
local seller_avail_key = KEYS[2]

local cost = tonumber(ARGV[1])

-- 1. Deduct cost from Buyer's held cash
local current_hold = tonumber(redis.call("GET", buyer_hold_key) or "0")
if current_hold >= cost then
    redis.call("DECRBY", buyer_hold_key, cost)
else
    -- In a perfect system this never happens due to upfront Risk Engine checks,
    -- but we prevent negative balances just in case.
    redis.call("SET", buyer_hold_key, 0) 
end

-- 2. Add cash to Seller's available balance
redis.call("INCRBY", seller_avail_key, cost)

return 1
`

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

	// 2. Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"), // e.g., "redis:6379"
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatal("❌ Failed to connect to Redis:", err)
	}
	fmt.Println("✅ Connected to Redis for Settlement")
	

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

		fmt.Printf("✅ Saved Trade: %s buys from %s at %d\n", trade.BuyOrderID, trade.SellOrderID, trade.Price)

		// 4. Settle Funds in Redis
		// Note: To do this perfectly, your Trade struct from Kafka needs the actual UserIDs, 
		// but since you currently only have OrderIDs in the Trade struct, you can parse them 
		// if your OrderIDs look like "user1_ord_123", or update the Rust engine to include buyer_user_id.
		
		// Assuming we extract the User IDs from the Order IDs:
		buyerUserID := extractUserID(trade.BuyOrderID) 
		sellerUserID := extractUserID(trade.SellOrderID)
		
		buyerHoldKey := fmt.Sprintf("user:%s:usd:hold", buyerUserID)
		sellerAvailKey := fmt.Sprintf("user:%s:usd:available", sellerUserID)
		totalCost := int64(trade.Price * trade.Quantity)

		err = rdb.Eval(context.Background(), settleTradeScript, []string{buyerHoldKey, sellerAvailKey}, totalCost).Err()
		if err != nil {
			log.Printf("🚨 CRITICAL: Redis Settlement Failed for Trade %s/%s: %v\n", trade.BuyOrderID, trade.SellOrderID, err)
		} else {
			log.Printf("💸 Settled $%d between %s and %s\n", totalCost, buyerUserID, sellerUserID)
		}
	}
}

func extractUserID(orderID string) string {
	// Example: "bot_6_ord_0_1774972706574581"
	// strings.Split creates an array: ["bot_6", "_0_1774972706574581"]
	parts := strings.Split(orderID, "_ord")
	
	if len(parts) > 0 {
		return parts[0] // Returns "bot_6"
	}
	
	// Fallback just in case the format is weird
	return orderID 
}