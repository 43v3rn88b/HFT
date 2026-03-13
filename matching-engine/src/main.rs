mod engine; // Tells Rust to look for the engine folder

// use engine::book::LimitOrderBook;
// use engine::types::{EngineCommand, Order};
use engine::types::Order;
use rdkafka::consumer::{Consumer, StreamConsumer};
use rdkafka::ClientConfig;
use rdkafka::Message;
use std::sync::mpsc::Sender;
use tokio;
use std::env;
use crate::engine::types::EngineCommand;
// Assuming `EngineCommand` and `spawn_matching_engine()` are imported from your engine module
//  use crate::engine::{EngineCommand, spawn_matching_engine};
use crate::engine::types::spawn_matching_engine;

#[tokio::main]
async fn main() {
    
    println!("Booting up High-Frequency Trading System...");

    // 1. Boot up the CPU-bound matching engine on its own OS thread
    let engine_tx: Sender<EngineCommand> = spawn_matching_engine();

    // --- NEW LOGIC: Read Kafka Broker from Environment ---
    // Read the variable from docker-compose.yml or default to the service name
    let kafka_broker = env::var("KAFKA_BROKER").unwrap_or_else(|_| "redpanda:9092".to_string());
    println!("Connecting to Kafka Broker: {}", kafka_broker);

    // 2. Configure the Kafka Consumer
    let consumer: StreamConsumer = ClientConfig::new()
        .set("group.id", "matching-engine-group")
        .set("bootstrap.servers", &kafka_broker) // Connects to Redpanda from our Docker Compose
        .set("enable.partition.eof", "false")
        .set("session.timeout.ms", "6000")
        .set("enable.auto.commit", "true") // In a real HFT, you'd commit manually AFTER matching
        .create()
        .expect("Failed to create Kafka consumer");

    // 3. Subscribe to the orders topic for Apple stock
    consumer.subscribe(&["orders.aapl"])
        .expect("Can't subscribe to specified topic");

    println!("🎧 Kafka Consumer listening on 'orders.aapl'...");

    // 4. The Async Event Loop (I/O Bound)
    loop {
        // Wait asynchronously for a message from Kafka. 
        // This does NOT block our Engine thread!
        match consumer.recv().await {
            Err(e) => eprintln!("Kafka error: {}", e),
            Ok(msg) => {
                // Extract the JSON payload bytes
                if let Some(payload) = msg.payload() {
                    
                    // Parse the JSON into our Rust `Order` struct
                    match serde_json::from_slice::<Order>(payload) {
                        Ok(order) => {
                            // Send it to the blazing-fast Engine thread
                            if let Err(e) = engine_tx.send(EngineCommand::PlaceOrder(order)) {
                                eprintln!("Fatal error: Engine channel disconnected! {}", e);
                                break;
                            }
                        }
                        Err(e) => {
                            eprintln!("Invalid JSON received from Kafka: {}", e);
                            // Drop it or send to a Dead Letter Queue (DLQ)
                        }
                    }
                }
            }
        }
    }
}
