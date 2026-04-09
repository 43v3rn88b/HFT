# ⚡ Distributed High-Frequency Trading (HFT) Engine

A highly performant, polyglot trading engine and live market terminal. This project implements a distributed microservices architecture mirroring tier-1 cryptocurrency exchanges and modern fintech platforms, capable of processing thousands of orders per second with microsecond latency.

## 🏗 Architecture & Tech Stack

This system is built using an event-driven architecture, separating the risk/gateway layer, the matching engine, and the settlement layer for maximum throughput and scalability.

* **Frontend (React / Next.js / Tailwind CSS):** A high-performance trading terminal featuring live lightweight-charts, real-time order book depth visualization, and sub-millisecond WebSocket batch processing for time-and-sales ticking.
* **API Gateway (Go / Gin / WebSockets):** Handles incoming REST/WebSocket connections, routing, and batching.
* **Risk Engine (Redis):** Executes atomic Lua scripts to guarantee strict zero-balance checks and instantly lock funds before orders reach the matching engine.
* **Message Broker (Kafka / Redpanda):** The distributed nervous system of the platform, handling order ingestion (`orders.aapl`) and trade execution (`trades.aapl`) streams with high-throughput durability.
* **Matching Engine (Rust):** A blazing-fast, memory-optimized matching engine utilizing `BTreeMap` structures for precise price-time-priority limit order execution.
* **Settlement Worker (Go / PostgreSQL):** Asynchronously consumes executed trades from Kafka to settle final balances and persist durable trade history into a relational database.

## ✨ Key Features

* **Microsecond Order Matching:** Rust-based BTree order book achieves extreme performance for high-frequency limits and market orders.
* **Event-Driven & Decoupled:** Components communicate strictly via Kafka topics, allowing the Rust engine to match orders independently of database write latency.
* **Zero-Double-Spend Guarantee:** Pre-trade risk checks utilize Redis atomic operations to ensure users can never spend balances they do not have.
* **High-Volume UI Rendering:** The React frontend utilizes batched updates, requestAnimationFrame, and array-aware WebSocket handlers to render 10,000+ load-test events per second without crashing.
* **Polyglot Microservices:** Best-in-class languages chosen for specific tasks (Go for concurrency/networking, Rust for raw compute, JS/React for UI).

## 🚀 Getting Started

Ensure you have Docker and Docker Compose installed.

```bash
# 1. Boot up the infrastructure (Redpanda, Redis, Postgres)
docker compose up -d

# 2. Start the microservices
docker compose up --build hft-matching-engine api-gateway settlement-worker -d

# 3. Launch the Trading Terminal
cd frontend
npm install
npm run dev
