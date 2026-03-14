package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/gorilla/websocket"
)

const luaCheckScript = `
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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local testing
	},
}

// This is the handler for GET /ws/orderbook
func serveWs(hub *Hub, c *gin.Context) {
	// Upgrade initial GET request to a websocket
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Register the new client with the Hub
	hub.AddClient(ws)
	defer hub.RemoveClient(ws)

	// Keep the connection open and listen for close events from the client
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break // Client disconnected
		}
	}
}

// 1. The Data Model
type Order struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Ticker   string `json:"ticker"`
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

	r := gin.Default()
	
	// Initialize the Hub and start the broadcaster goroutine
	hub := NewHub()
	go hub.Run()

	go StartOrderBookConsumer(hub)

	// Add the new websocket route
	// r.GET("/ws/orderbook", func(c *gin.Context) {
	// 	serveWs(hub, c)
	// })

	// 1. UPDATE THIS: Read from the Docker environment variable
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "redis:6379" // Fallback
	}

	// Connect to Redis (from our Docker Compose)
	rdb = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// 2. UPDATE THIS: Read Kafka broker from environment
	kafkaAddr := os.Getenv("KAFKA_BROKER")
	if kafkaAddr == "" {
		kafkaAddr = "hft-redpanda:9092" // Fallback
	}

	// Connect to Kafka / Redpanda
	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(kafkaAddr),
		Topic:    "orders.aapl",
		Balancer: &kafka.LeastBytes{},
	}

	// Setup the HTTP Router
	// router := gin.Default()
	r.POST("/orders", handlePlaceOrder)
	r.GET("/ws/orderbook", func(c *gin.Context) {
		serveWs(hub, c)
	})

	// Start listening for web traffic
	// r.Run(":8080")
	// 3. Start the server (This MUST be the absolute last line in main)
	if err := r.Run(":8080"); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
// Paste this at the bottom of main.go
	func StartOrderBookConsumer(hub *Hub) {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  []string{"hft-redpanda:9092"}, // Assuming Docker networking
			Topic:    "orderbook.aapl",
			GroupID:  "api-gateway-ws-group",
			MinBytes: 10e3, // 10KB
			MaxBytes: 10e6, // 10MB
		})

		log.Println("Order Book Consumer started. Listening for Redpanda messages...")

		for {
			m, err := reader.ReadMessage(context.Background())
			if err != nil {
				log.Printf("Error reading from Redpanda: %v", err)
				break
			}
			// Push the raw JSON bytes directly into the WebSocket Hub
			hub.Broadcast <- m.Value
		}
	}
func handlePlaceOrder(c *gin.Context) {
	var order Order

	// 1. Parse the incoming JSON request
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	var availableKey string
    var holdKey string
    var requiredAmount uint64

    if order.Side == "Buy" {
        // Buyers must have enough USD (Price * Quantity)
        availableKey = fmt.Sprintf("user:%s:usd:available", order.UserID)
        holdKey = fmt.Sprintf("user:%s:usd:hold", order.UserID)
        requiredAmount = order.Price * order.Quantity
    } else {
        // Sellers must have enough SHARES (Quantity only)
        // We lowercase the ticker to match our Redis keys
        tickerLower := strings.ToLower(order.Ticker)
        availableKey = fmt.Sprintf("user:%s:%s:available", order.UserID, tickerLower)
        holdKey = fmt.Sprintf("user:%s:%s:hold", order.UserID)
        requiredAmount = order.Quantity
    }
	// 2. Calculate the total cost of the trade
	// tradeCost := order.Price * order.Quantity

	// 3. Define the Redis keys for this specific user
	// availableKey := fmt.Sprintf("user:%s:usd:available", order.UserID)
	// holdKey := fmt.Sprintf("user:%s:usd:hold", order.UserID)

	// 4. Run the Lua Script in Redis (The Atomic Lock)
	// This happens in <1 millisecond
	result, err := rdb.Eval(ctx, luaCheckScript, []string{availableKey, holdKey}, requiredAmount).Result()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Risk engine failure", 
            "details": err.Error(),})
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
		// --- NEW COMPENSATION LOGIC ---
		fmt.Printf("Kafka failed! Rolling back Redis for user %s\n", order.UserID)
		
		// Move money back: Decrease Hold, Increase Available
		rollbackScript := `
			redis.call('DECRBY', KEYS[2], ARGV[1])
			redis.call('INCRBY', KEYS[1], ARGV[1])
			return 1
		`
		rdb.Eval(ctx, rollbackScript, []string{availableKey, holdKey}, requiredAmount)
		
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