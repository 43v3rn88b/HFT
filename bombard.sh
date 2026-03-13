#!/bin/bash

echo "🚀 Firing 100 concurrent orders at the API Gateway..."

# Loop 100 times
for i in {1..100}
do
  # The '&' at the end pushes the curl command to the background
  # so they all fire at almost the exact same time
  curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "id": "burst-order-'$i'",
    "user_id": "trader-99",
    "ticker": "AAPL",
    "side": "Buy",
    "price": 15000,
    "quantity": 1
  }' > /dev/null & 
done

# Wait for all background jobs to finish
wait
echo "✅ All 100 orders sent!"
