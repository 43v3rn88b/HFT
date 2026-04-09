package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/gin-contrib/cors"
)

// var upgrader = websocket.Upgrader{
// 	CheckOrigin: func(r *http.Request) bool {
// 		return true // Allow all origins for local dev
// 	},
// }

// 2. Client Hub for Trades
var (
	tradeClients = make(map[*websocket.Conn]bool)
	tradeMutex   = sync.Mutex{}
)

// 3. The Gin Route Handler
func wsTradesHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("❌ WebSocket Upgrade Error:", err)
		return
	}
	defer ws.Close()

	// Register new client
	tradeMutex.Lock()
	tradeClients[ws] = true
	tradeMutex.Unlock()

	log.Println("📈 New client connected to Live Trades stream")

	// Keep connection alive until client disconnects
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			tradeMutex.Lock()
			delete(tradeClients, ws)
			tradeMutex.Unlock()
			break
		}
	}
}

func startTradeBroadcaster(brokerURL string) {
	// Connect to the Redpanda/Kafka broker
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{brokerURL}, // Adjust if your broker address is different
		Topic:     "trades.aapl",
		GroupID: "gateway-trades-group",
		// Partition: 0,
		// MinBytes:  10e3, // 10KB
		// MaxBytes:  10e6, // 10MB
	})

	// 1. Create a channel to catch incoming Kafka trades
	tradeChan := make(chan Trade, 5000)

	log.Println("🎧 Go Gateway listening to trades.aapl...")

	go func() {
		for {
			m, err := r.ReadMessage(context.Background())
			if err != nil {
				// log.Println("❌ Kafka Consumer Error:", err)
				// Add a small pause so it doesn't spam your terminal in an infinite loop
                time.Sleep(1 * time.Second)
				continue
			}
			var trade Trade
			json.Unmarshal(m.Value, &trade)
			tradeChan <- trade
		}
	}()
	// 3. The Throttle: Broadcast to WebSockets exactly 10 times per second
	ticker := time.NewTicker(100 * time.Millisecond)
	var batch []Trade

	for {
		select {
		case trade := <-tradeChan:
			// Add to our bucket
			batch = append(batch, trade)

		case <-ticker.C:
			// Every 100ms, if we have trades, send them as ONE array
			if len(batch) > 0 {
				broadcastData, _ := json.Marshal(batch)
				
				// Broadcast to all connected WebSocket clients
				tradeMutex.Lock()
				for client := range tradeClients {
					err := client.WriteMessage(websocket.TextMessage, broadcastData)
					if err != nil {
						log.Printf("Disconnected client. Error: %v", err)
						client.Close()
						delete(tradeClients, client)
					}
				}
				tradeMutex.Unlock()
				
				// Empty the bucket for the next 100ms cycle
				batch = nil 
			}
		}
	}
}

func startKafkaConsumer(brokerURL string, topic string) {
    // 1. Initialize the Reader
    r := kafka.NewReader(kafka.ReaderConfig{
        Brokers:  []string{brokerURL}, // Or your KAFKA_BROKER env var
        Topic:    topic,
        GroupID:  "gateway-ui-group", // New group so it doesn't interfere with others
        MaxBytes: 10e6, // 10MB
    })

    log.Printf("👂 Listening to Kafka topic: %s", topic)

    for {
        // 2. Read the message (this blocks until a message arrives)
        m, err := r.ReadMessage(context.Background())
        if err != nil {
            log.Printf("❌ Kafka Read Error on %s: %v", topic, err)
            break
        }

        // 3. Send the raw JSON bytes directly to the WebSocket broadcast channel
        broadcast <- m.Value
    }
}

// A map to hold all active browser connections
var clients = make(map[*websocket.Conn]bool) 
// A channel where we will send our trade/orderbook JSON updates
var broadcast = make(chan []byte)

// 1. Upgrade the HTTP request to a WebSocket connection
func handleConnections(c *gin.Context) {
	log.Println("📥 DEBUG: Someone is trying to connect to /ws...")
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("❌ WebSocket Upgrade Error: %v", err)
		return
	}
	defer ws.Close()

	clients[ws] = true
	log.Println("🔌 New UI Client Connected to Live Feed!")

	// Infinite loop to keep the connection alive
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			log.Println("🔌 Client Disconnected.", err)
			delete(clients, ws)
			break
		}
	}
}

// 2. The Broadcast Loop (Runs in the background)
func handleMessages() {
	for {
		// Wait for a new message to arrive on the broadcast channel
		msg := <-broadcast
		
		// Blast it out to every connected browser
		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				client.Close()
				delete(clients, client)
			}
		}
	}
}


// 1. The Data Model
type Order struct {
	Action   string `json:"action"`
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Ticker   string `json:"ticker"`
	Side     string `json:"side"`  // "Buy" or "Sell"
	Price    uint64 `json:"price"` // In cents
	Quantity uint64 `json:"quantity"`
}

type Trade struct {
	BuyOrderID  string `json:"buy_order_id"`
	SellOrderID string `json:"sell_order_id"`
	Ticker      string `json:"ticker"`
	Price       uint64 `json:"price"`
	Quantity    uint64 `json:"quantity"`
}

type CancelOrderRequest struct {
    OrderID string `json:"order_id"`
    Ticker  string `json:"ticker"`
    Side    string `json:"side"` // Optional, but helpful for the engine to find it faster
}

// 2. The Atomic Lua Script (The Risk Engine)
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

var ctx = context.Background()
var rdb *redis.Client
var kafkaWriter *kafka.Writer

// --- 🚨 NEW ORDER BOOK BROADCASTER 🚨 ---
var (
	obClients = make(map[*websocket.Conn]bool)
	obMutex   = sync.Mutex{}
)

// The Gin Route Handler
func wsOrderBookHandler(c *gin.Context) {
	ws, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("❌ Order Book WS Error:", err)
		return
	}
	defer ws.Close()

	obMutex.Lock()
	obClients[ws] = true
	obMutex.Unlock()

	log.Println("📊 New UI client connected to Order Book stream")

	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			obMutex.Lock()
			delete(obClients, ws)
			obMutex.Unlock()
			break
		}
	}
}

// The Kafka Consumer
func startOrderBookBroadcaster(brokerURL string) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{brokerURL},
		Topic:   "snapshots.aapl", // 🚨 Matches Rust exactly!
		GroupID: "gateway-ob-group",
	})

	log.Println("🎧 Go Gateway listening to snapshots.aapl...")

	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		obMutex.Lock()
		for client := range obClients {
			if err := client.WriteMessage(websocket.TextMessage, m.Value); err != nil {
				client.Close()
				delete(obClients, client)
			}
		}
		obMutex.Unlock()
	}
}

func main() {
	fmt.Println("🚀 Starting API Gateway & Risk Engine...")

	kafkaAddr := os.Getenv("KAFKA_BROKER")
	if kafkaAddr == "" {
		kafkaAddr = "localhost:19092" // Pointing to your Docker Redpanda
	}

	// 1. Start the broadcaster in a background thread
    go handleMessages()

	go startTradeBroadcaster(kafkaAddr)
	go startOrderBookBroadcaster(kafkaAddr)

	 // 2. Open the WebSocket route
    // http.HandleFunc("/ws", handleConnections)

	// Start the background Kafka consumer
	// startTradeBroadcaster()
	
	
    log.Println("✅ Gateway is live on :8082")
    // http.ListenAndServe(":8082", nil)

   

	// --- 1. INITIALIZE INFRASTRUCTURE ---
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Assuming local testing for now
	}
	rdb = redis.NewClient(&redis.Options{Addr: redisAddr})

	
	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(kafkaAddr),
		Topic:    "orders.aapl",
		Balancer: &kafka.LeastBytes{},

		// --- THE PERFORMANCE PUSH ---
    // Wait until we have 100 messages OR 5 milliseconds have passed
    BatchSize:    100, 
    BatchTimeout: 5 * time.Millisecond, 
    
    // Fire and forget. Hands the message to the background buffer 
    // and immediately returns to the HTTP handler.
    Async:        false,
	// Optional: Add a timeout so it doesn't hang forever if broken
    WriteTimeout: 2 * time.Second,
	}

	// --- 4. SETUP GIN ROUTER ---
	r := gin.Default()
	r.SetTrustedProxies(nil)
	r.Use(cors.Default())

	// Add this line to map the DELETE request to your handler
	r.DELETE("/orders/:ticker/:id", handleCancelOrder)

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"}, // Your Next.js frontend
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour, // Caches the preflight request so it doesn't slow down trading
	}))
	
	
	// r.DELETE("/orders/:ticker/:id", func(c *gin.Context) {
    //     ticker := c.Param("ticker")
    //     orderID := c.Param("id")
        
    //     log.Printf("🗑️ Cancellation received for: %s | ID: %s", ticker, orderID)
        
    //     // Your logic to push to Kafka/Redis here
        
    //     c.JSON(200, gin.H{"message": "Order cancellation sequenced", "order_id": orderID})
    // })

	
	// ADD THIS CORS MIDDLEWARE block right here:
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		// 🚨 THE CRITICAL LINE: Make sure DELETE and OPTIONS are here!
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, PUT, OPTIONS")
		
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// REST Endpoint: Place an Order
	r.POST("/orders", handlePlaceOrder)

	r.GET("/ws/orderbook", wsOrderBookHandler)
	
	// WebSocket Endpoint: Stream the Order Book
	// r.GET("/ws/orderbook", func(c *gin.Context) {
	// 	// We pass the Gin request directly into our WSManager
	// 	wsManager.HandleConnection(c.Writer, c.Request)
	// })

	// Register the WebSocket route specifically with Gin!
	r.GET("/ws", handleConnections)
	r.GET("/ws/trades", wsTradesHandler)

	// --- 5. START SERVER ---
	log.Println("✅ Go API Gateway fully running on port 8082")
	if err := r.Run("0.0.0.0:8082"); err != nil {
		log.Fatal("Server failed to start:", err)
	}
	// r.Run(":8082")
}

func handleCancelOrder(c *gin.Context) {
	ticker := c.Param("ticker")
	orderID := c.Param("id")
	userID := "user123" // In reality, extract this from a JWT

	// Create the explicit Cancel payload
	// cmd := Order{
	// 	Action: "cancel",
	// 	ID:     orderID,
	// 	Ticker: ticker,
	// 	UserID: userID,
	// }

	// 1. Create the cancel event
    // cancelEvt := CancelOrderRequest{
    //     OrderID: orderID,
    //     Ticker:  ticker,
    // }

	// 1. Construct the explicit Command struct expected by Rust
	cancelCmd := struct {
		Action   string `json:"action"`
		ID       string `json:"id"`
		UserID   string `json:"user_id"`
		Ticker   string `json:"ticker"`
		Side     string `json:"side"`     // Rust might require these fields to exist
		Price    int    `json:"price"`    // even if they are empty/zero for a cancel
		Quantity int    `json:"quantity"`
	}{
		Action:   "cancel",
		ID:       orderID,
		UserID:   userID,
		Ticker:   ticker,
		Side:     "", 
		Price:    0,
		Quantity: 0,
	}

	// 2. Convert to JSON
    payload, _ := json.Marshal(cancelCmd)

	// cmdBytes, _ := json.Marshal(cmd)

	// 3. Push to the 'orders.cancel' Kafka topic
    // Assuming you have a kafkaWriter defined globally
    err := kafkaWriter.WriteMessages(context.Background(),
        kafka.Message{
            Key:   []byte(orderID),
            Value: payload,
        },
    )

	// Sequence it into Kafka so the Rust engine processes it in strict order
	// err := kafkaWriter.WriteMessages(ctx, kafka.Message{
	// 	Key:   []byte(orderID),
	// 	Value: cmdBytes,
	// })

	if err != nil {
		fmt.Printf("❌ KAFKA WRITE FAILED: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sequence cancellation"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "Cancel request sequenced", "id": orderID})
}

// Don't forget to update your handlePlaceOrder to set `Action: "place"` 
// when marshaling the JSON for Kafka!

// REST API Handler for incoming trades
func handlePlaceOrder(c *gin.Context) {
	var order Order

	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	var availableKey string
	var holdKey string
	var requiredAmount uint64

	if order.Side == "Buy" {
		availableKey = fmt.Sprintf("user:%s:usd:available", order.UserID)
		holdKey = fmt.Sprintf("user:%s:usd:hold", order.UserID)
		requiredAmount = order.Price * order.Quantity
	} else {
		tickerLower := strings.ToLower(order.Ticker)
		availableKey = fmt.Sprintf("user:%s:%s:available", order.UserID, tickerLower)
		holdKey = fmt.Sprintf("user:%s:%s:hold", order.UserID, tickerLower)
		requiredAmount = order.Quantity
	}

	// Execute Redis Lua Script
	result, err := rdb.Eval(ctx, luaCheckScript, []string{availableKey, holdKey}, requiredAmount).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Risk engine failure", "details": err.Error()})
		return
	}

	if result.(int64) == 0 {
		// c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient funds"})
		// return
		fmt.Println("⚠️ Bypassing Redis zero-balance check for load test!")
	}

	// 1. Wrap the order in our new Command struct
	cmd := Order{
		Action:   "place", // Explicitly tag this as a place command
		ID:       order.ID,
		UserID:   order.UserID,
		Ticker:   order.Ticker,
		Side:     order.Side,
		Price:    order.Price,
		Quantity: order.Quantity,
	}
	// Send to Kafka
	orderBytes, _ := json.Marshal(cmd)
	err = kafkaWriter.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(order.ID),
		Value: orderBytes,
	})

	if err != nil {
		fmt.Printf("❌ Kafka failed! Error: %v | Rolling back Redis for user %s\n", err, order.UserID)
		fmt.Printf("❌ Kafka failed! Rolling back Redis for user %s\n", order.UserID)
		rollbackScript := `
			redis.call('DECRBY', KEYS[2], ARGV[1])
			redis.call('INCRBY', KEYS[1], ARGV[1])
			return 1
		`
		rdb.Eval(ctx, rollbackScript, []string{availableKey, holdKey}, requiredAmount)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sequence order"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Order accepted and sequenced",
		"order_id": order.ID,
	})
}
