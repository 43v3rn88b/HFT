mod engine;

use engine::types::{Order, Trade, EngineCommand, IncomingCommand};
use engine::spawn_matching_engine; 
use rdkafka::consumer::{Consumer, StreamConsumer};
use rdkafka::producer::{FutureProducer, FutureRecord};
use rdkafka::ClientConfig;
use rdkafka::Message;
use std::sync::mpsc::{channel, Sender, Receiver};
use tokio;
use std::env; 
use std::time::Duration;
use crate::engine::types::CancelOrderRequest;
use rdkafka::util::Timeout; // <-- ADD THIS

#[tokio::main]
async fn main() {
    println!("Booting up High-Frequency Trading System...");

    let kafka_broker = env::var("KAFKA_BROKER").unwrap_or_else(|_| "redpanda:9092".to_string());
    println!("Connecting to Kafka Broker: {}", kafka_broker);

    // 1. Create Producers
    let trade_producer: FutureProducer = ClientConfig::new()
        .set("bootstrap.servers", &kafka_broker)
        .set("message.timeout.ms", "5000")
        .create()
        .expect("Failed to create Trade producer");

    let snapshot_producer: FutureProducer = ClientConfig::new()
        .set("bootstrap.servers", &kafka_broker)
        .set("message.timeout.ms", "5000")
        .create()
        .expect("Failed to create Snapshot producer");

    let dlq_producer: FutureProducer = ClientConfig::new()
        .set("bootstrap.servers", &kafka_broker)
        .set("message.timeout.ms", "5000")
        .create()
        .expect("Failed to create DLQ producer");

    // 2. Setup Channels
    let (trade_tx, trade_rx): (Sender<Trade>, Receiver<Trade>) = channel();
    let (snap_tx, snap_rx): (Sender<String>, Receiver<String>) = channel();

    // 3. Boot up the Engine
    let engine_tx: Sender<EngineCommand> = spawn_matching_engine(trade_tx, snap_tx);

    // 4. Task: Publish Trades
    tokio::spawn(async move {
        while let Ok(trade) = trade_rx.recv() {
            // LOUD PRINT: This confirms the engine actually matched something
            println!("💰 MATCH EXECUTED: {} shares of {} at ${}", trade.quantity, trade.ticker, trade.price);

            let payload = serde_json::to_string(&trade).unwrap();
            let _ = trade_producer.send(
                FutureRecord::to("trades.aapl").payload(&payload).key(&trade.ticker),
                Duration::from_secs(0)
            ).await;
        }
    });

    // 5. Task: Publish OrderBook Snapshots
    tokio::spawn(async move {
        while let Ok(snapshot_json) = snap_rx.recv() {
            let _ = snapshot_producer.send(
                FutureRecord::to("snapshots.aapl").payload(&snapshot_json).key("AAPL"),
                Duration::from_secs(0)
            ).await;
        }
    });

    // 6. Consumer Loop
    let consumer: StreamConsumer = ClientConfig::new()
        .set("group.id", "matching-engine-group-2")
        .set("bootstrap.servers", &kafka_broker)
        .set("enable.auto.commit", "true")
        .set("auto.offset.reset", "earliest")
        .create()
        .expect("Failed to create Kafka consumer");

    consumer.subscribe(&["orders.aapl"]).expect("Subscription failed");


    
    
    println!("🎧 Kafka Consumer listening on 'orders.aapl'...");
    loop {
        match consumer.recv().await {
            Ok(msg) => {
                // Get the topic name to route the message correctly
                let topic = msg.topic();

                if let Some(payload) = msg.payload() {
                    // 1. Parse the incoming order
                    let order: Order = serde_json::from_slice(payload).unwrap();
                    
                    match topic {
                        // 🟢 Topic for New Orders
                        "orders.aapl" => {
                           // Try to parse. If it fails, YELL LOUDLY.
                            match serde_json::from_slice::<IncomingCommand>(payload) {
                                Ok(command) => {
                                    match command {
                                        IncomingCommand::Place(order) => {
                                            println!("✅ PLACE COMMAND: ID: {}, Side: {:?}", order.id, order.side);
                                            let _ = engine_tx.send(EngineCommand::PlaceOrder(order));
                                        },
                                        IncomingCommand::Cancel { id, ticker, .. } => {
                                            println!("🗑️ Engine routing cancel request for: {}", id);
                                            let _ = engine_tx.send(EngineCommand::CancelOrder { ticker, id });
                                        }
                                    }
                                },
                                Err(e) => {
                                    // THIS will save you hours of debugging in the future!
                                    let raw_json = String::from_utf8_lossy(payload);
                                    eprintln!("❌ JSON PARSE FAILED: {} | Raw Payload: {}", e, raw_json);
                                }
                            }
                        },
                        _ => println!("❓ Received message from unknown topic: {}", topic),
                
                        
                        // Err(e) => {
                        //     // 1. Log the exact reason why parsing failed
                        //     eprintln!("❌ PARSE FAILURE: {} | Raw: {}", e, raw_string);

                        //     // 2. Move the original payload to the DLQ so it doesn't block the engine
                        //     let _ = dlq_producer.send(
                        //         FutureRecord::to("orders.aapl.dlq")
                        //             .payload(payload)
                        //             .key("invalid"),
                        //         Duration::from_secs(0)
                        //     ).await;
                        // }
                    }
                }
            }
            Err(e) => eprintln!("Kafka error: {}", e),
        }
    }
}