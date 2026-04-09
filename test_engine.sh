#!/bin/bash

# Base URL (Assuming you run this script from inside WSL)
API="http://localhost:8082/orders"

echo "🧹 Please restart your Go and Rust servers to clear in-memory state."
read -p "Press [Enter] when servers are running and UI is open..."

echo "----------------------------------------"
echo "🧪 TEST 1: Building the Book (No Matches)"
echo "----------------------------------------"
# 1. Buy 10 @ $10.00
curl -X POST $API -H "Content-Type: application/json" -d '{"id": "t1_b1", "user_id": "u1", "ticker": "AAPL", "side": "Buy", "price": 1000, "quantity": 10}'
sleep 2

# 2. Sell 10 @ $20.00 (Should not match, spread is $10 to $20)
curl -X POST $API -H "Content-Type: application/json" -d '{"id": "t1_s1", "user_id": "u2", "ticker": "AAPL", "side": "Sell", "price": 2000, "quantity": 10}'
sleep 2

echo "👀 CHECK UI: You should see 10 @ $10.00 in Bids (Green) and 10 @ $20.00 in Asks (Red)."
read -p "Press [Enter] to continue..."

echo "----------------------------------------"
echo "🧪 TEST 2: The Exact Match"
echo "----------------------------------------"
# 3. Sell 5 @ $10.00 (Should instantly match half of our resting Bid)
curl -X POST $API -H "Content-Type: application/json" -d '{"id": "t2_s1", "user_id": "u3", "ticker": "AAPL", "side": "Sell", "price": 1000, "quantity": 5}'
sleep 2

echo "👀 CHECK UI: 'Live Trades' should show 5 @ $10.00. The Bid book should drop from 10 to 5."
read -p "Press [Enter] to continue..."

echo "----------------------------------------"
echo "🧪 TEST 3: Price Priority (Crossing the Spread)"
echo "----------------------------------------"
# We have a resting Sell of 10 @ $20.00.
# Someone comes in willing to buy at $150.00! The engine MUST give them the $20.00 price.
curl -X POST $API -H "Content-Type: application/json" -d '{"id": "t3_b1", "user_id": "u4", "ticker": "AAPL", "side": "Buy", "price": 15000, "quantity": 10}'
sleep 2

echo "👀 CHECK UI: 'Live Trades' MUST show a trade at $20.00 (Not $150!). The Ask book should be empty."
echo "----------------------------------------"
echo "✅ Tests Complete."