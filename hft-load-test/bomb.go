package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// Adjust these to match the port in your docker-compose.yml
const apiGatewayURL = "http://localhost:8082/orders"//"http://172.30.85.187:8082/orders"
const numBots = 10
const ordersPerBot = 1000

type OrderPayload struct {
	Action   string `json:"action"`
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Ticker   string `json:"ticker"`
	Side     string `json:"side"`
	Price    int    `json:"price"` // in cents
	Quantity int    `json:"quantity"`
}

func main() {
	fmt.Println("🚀 Initializing HFT Load Test...")
	
	// 1. AUTO-FUND BOT ACCOUNTS IN REDIS
	fundBots()

	// 2. WARM UP THE ENGINE
	fmt.Println("🔥 Unleashing the swarm in 3 seconds...")
	time.Sleep(3 * time.Second)

	startTime := time.Now()
	var wg sync.WaitGroup

	// 3. LAUNCH CONCURRENT BOTS
	for i := 1; i <= numBots; i++ {
		wg.Add(1)
		go func(botID int) {
			defer wg.Done()
			userID := fmt.Sprintf("bot_%d", botID)
			client := &http.Client{Timeout: 5 * time.Second}

			for j := 0; j < ordersPerBot; j++ {
				// Randomize order to create a chaotic market
				side := "Buy"
				price := 15500 // Base price $150.00
				if rand.Float32() > 0.5 {
					side = "Sell"
					// Sellers panic and ask for exactly $149.00
        			price = 14500
				}
				// else {
				// 	// Buyers are aggressive and bid exactly $151.00
				// 	price = 15100 
				// }
				
				// Keep prices tight around $150.00 (15000 cents) to force matches
				// price := 14950 + rand.Intn(100) 

				// Force prices to overlap significantly
				// Buyers will bid up to $155, Sellers will ask down to $145.
				// This guarantees thousands of matches.
				// price := 14500 + rand.Intn(1000)
				quantity := 1 + rand.Intn(10)

				payload := OrderPayload{
					Action:   "place",
					// Add time.Now().UnixMicro() to guarantee a unique ID every single run!
					ID:       fmt.Sprintf("%s_ord_%d_%d", userID, j, time.Now().UnixMicro()),
					UserID:   userID,
					Ticker:   "AAPL",
					Side:     side,
					Price:    price,
					Quantity: quantity,
				}

				jsonData, _ := json.Marshal(payload)
				req, _ := http.NewRequest("POST", apiGatewayURL, bytes.NewBuffer(jsonData))
				req.Header.Set("Content-Type", "application/json")

				resp, err := client.Do(req)
				// if err == nil {
				// 	resp.Body.Close()
				// }else {
				// 	// 🚨 THIS IS THE TRUTH TELLER 🚨
				// 	if resp.StatusCode != 200 && resp.StatusCode != 201 {
				// 		fmt.Printf("⚠️ Bot %d rejected! Status: %d\n", botID, resp.StatusCode)
				// 	}
				// 	resp.Body.Close()
				// }
				if err != nil {
					log.Fatalf("Fatal HTTP error: %v", err)
				}
				// defer resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					body, _ := io.ReadAll(resp.Body)
					log.Fatalf("❌ SERVER REJECTED ORDER! Status: %d, Body: %s", resp.StatusCode, string(body))
				}

				// Add this to catch 404s, 500s, or 400 Bad Requests!
				// if resp.StatusCode != http.StatusOK {
				// 	bodyBytes, _ := io.ReadAll(resp.Body)
				// 	log.Fatalf("Server rejected order! Status: %d, Body: %s", resp.StatusCode, string(bodyBytes))
				// }
				if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
					body, _ := io.ReadAll(resp.Body)
					log.Printf("❌ SERVER REJECTED ORDER! Status: %d, Body: %s", resp.StatusCode, string(body))
					return // Use return to stop this specific goroutine, not log.Fatalf which kills the whole program!
				}
				resp.Body.Close()
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)
	totalOrders := numBots * ordersPerBot

	fmt.Println("========================================")
	fmt.Printf("✅ LOAD TEST COMPLETE\n")
	fmt.Printf("Total Orders Submitted: %d\n", totalOrders)
	fmt.Printf("Time Taken: %v\n", duration)
	fmt.Printf("Throughput: %.2f orders/second\n", float64(totalOrders)/duration.Seconds())
	fmt.Println("========================================")
}

func fundBots() {
	rdb := redis.NewClient(&redis.Options{
		Addr: "172.30.85.187:6379",
	})
	ctx := context.Background()

	// Note: Adjust the key string below to match exactly how your Go API Risk Engine expects it!
	for i := 1; i <= numBots; i++ {
		userID := fmt.Sprintf("bot_%d", i)
		usdKey := fmt.Sprintf("balance:%s:USD", userID)
		aaplKey := fmt.Sprintf("balance:%s:AAPL", userID)

		// Give each bot $10,000,000 (in cents) and 10,000 shares
		rdb.Set(ctx, usdKey, 1000000000, 0)
		rdb.Set(ctx, aaplKey, 10000, 0)
	}
	fmt.Printf("💰 Auto-funded %d bot accounts in Redis.\n", numBots)
}