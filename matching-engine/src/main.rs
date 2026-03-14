mod engine; // Tells Rust to look for the engine folder

use engine::types::{Order, Trade, EngineCommand};
use engine::spawn_matching_engine; 
use rdkafka::consumer::{Consumer, StreamConsumer};
use rdkafka::producer::{FutureProducer, FutureRecord};
use rdkafka::ClientConfig;
use rdkafka::Message;
use std::sync::mpsc::{channel, Sender, Receiver}; // Fixed imports here
use tokio;
use std::env; 
use std::time::Duration;

#[tokio::main]
async fn main() {
    
    println!("Booting up High-Frequency Trading System...");

    // 1. Boot up the CPU-bound matching engine on its own OS thread
    // let engine_tx: Sender<EngineCommand> = spawn_matching_engine(std::sync::mpsc::Sender<Trade>);

    // --- NEW LOGIC: Read Kafka Broker from Environment ---
    // Read the variable from docker-compose.yml or default to the service name
    let kafka_broker = env::var("KAFKA_BROKER").unwrap_or_else(|_| "redpanda:9092".to_string());
    println!("Connecting to Kafka Broker: {}", kafka_broker);

    // 1. Create the Producer that will publish Trades
    let trade_producer: FutureProducer = ClientConfig::new()
        .set("bootstrap.servers", &kafka_broker)
        .set("message.timeout.ms", "5000")
        .create()
        .expect("Failed to create Trade producer");

    // 2. Create the Return Channel from the Engine
    let (trade_tx, trade_rx): (Sender<Trade>, Receiver<Trade>) = channel();

    // 3. Boot up the CPU-bound matching engine, passing it the return channel
    let engine_tx: Sender<EngineCommand> = spawn_matching_engine(trade_tx);

    // 4. Spawn a dedicated async task just to publish trades to Kafka
    // This runs independently so it never blocks incoming orders!
    tokio::spawn(async move {
        loop {
            // Wait for a trade from the engine
            if let Ok(trade) = trade_rx.recv() {
                println!("✅ Match found! Publishing Trade to Redpanda: {:?}", trade);
                
                let payload = serde_json::to_string(&trade).unwrap();
                let record = FutureRecord::to("trades.aapl")
                    .payload(&payload)
                    .key(&trade.ticker); // Keying by ticker ensures ordered processing
                    
                let _ = trade_producer.send(record, Duration::from_secs(0)).await;
            }
        }
    });

    let dlq_producer: FutureProducer = ClientConfig::new()
        .set("bootstrap.servers", &kafka_broker)
        .set("message.timeout.ms", "5000")
        .create()
        .expect("Failed to create DLQ producer");

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
                            eprintln!("Invalid JSON received. Moving to DLQ: {}", e);
                            
                            // Create a record targeting our new DLQ topic
                            let record = FutureRecord::to("orders.aapl.dlq")
                                .payload(payload)
                                .key("invalid-format");
                                
                            // Send it asynchronously without blocking the main event loop
                            let _ = dlq_producer.send(record, Duration::from_secs(0)).await;
                        }
                    }
                }
            }
        }
    }
}
