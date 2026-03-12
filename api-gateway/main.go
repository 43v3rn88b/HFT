package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

// 1. The Data Model
type Order struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Side     string `json:"side"` // "Buy" or "Sell"
	Price    uint64 `json:"price"` // In cents
	Quantity uint64 `json:"quantity"`
}

// 2. The Atomic Lua Script (The Risk Engine)
const riskEngineScript = `
local available_key = KEYS[1]
local hold_key = KEYS[2]
local trade_cost = tonumber(ARGV[1])

local current_available = tonumber(redis.call('GET', available_key) or '0')

if current_available >= trade_cost then
    redis.call('DECRBY', available_key, trade_cost)
    redis.call('INCRBY', hold_key, trade_cost)
    return 1 -- Success
else
    return 0 -- Insufficient Funds
end
`

var ctx = context.Background()
var rdb *redis.Client
var kafkaWriter *kafka.Writer

func main() {
	fmt.Println("Starting API Gateway & Risk Engine...")

	// Connect to Redis (from our Docker Compose)
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Connect to Kafka / Redpanda
	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP("localhost:19092"),
		Topic:    "orders.aapl",
		Balancer: &kafka.LeastBytes{},
	}

	// Setup the HTTP Router
	router := gin.Default()
	router.POST("/orders", handlePlaceOrder)

	// Start listening for web traffic
	router.Run(":8080")
}
func handlePlaceOrder(c *gin.Context) {
	var order Order

	// 1. Parse the incoming JSON request
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 2. Calculate the total cost of the trade
	tradeCost := order.Price * order.Quantity

	// 3. Define the Redis keys for this specific user
	availableKey := fmt.Sprintf("user:%s:usd:available", order.UserID)
	holdKey := fmt.Sprintf("user:%s:usd:hold", order.UserID)

	// 4. Run the Lua Script in Redis (The Atomic Lock)
	// This happens in <1 millisecond
	result, err := rdb.Eval(ctx, riskEngineScript, []string{availableKey, holdKey}, tradeCost).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Risk engine failure"})
		return
	}

	// 5. Check the result of the Lua script
	if result.(int64) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient funds"})
		return
	}

	// 6. IF WE GET HERE: The funds are successfully locked!
	// Now we serialize the order and push it to Kafka
	orderBytes, _ := json.Marshal(order)

	err = kafkaWriter.WriteMessages(ctx,
		kafka.Message{
			Key:   []byte(order.ID), // Partitions by Order ID
			Value: orderBytes,
		},
	)

	if err != nil {
		// DANGER: We locked the funds, but Kafka failed! 
		// In a production system, you would unlock the funds in Redis here before returning the error.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sequence order"})
		return
	}

	// 7. Success! Tell the user the order is pending execution.
	c.JSON(http.StatusCreated, gin.H{
		"message": "Order accepted and sequenced",
		"order_id": order.ID,
	})
}