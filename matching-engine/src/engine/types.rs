use std::collections::HashMap;
use std::sync::mpsc::{self, Receiver, Sender};
use std::thread;
use serde::Deserialize;
use std::collections::VecDeque;

// 1. The Core Data Types
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub enum Side {
    Buy,
    Sell,
}

#[derive(Debug, Clone, Deserialize)]
pub struct Order {
    pub id: String,
    pub user_id: String,
    pub ticker: String,
    pub side: Side,
    pub price: u64, // Use integers (cents) for prices to avoid floating-point math errors!
    pub quantity: u64,
}

// We added `pub` to the struct and its fields so book.rs can see it
#[derive(Debug)]
pub struct PriceLevel {
    pub price: u64,
    pub orders: VecDeque<Order>,
    pub total_volume: u64,
}

// We also need the implementation block so book.rs can create and update them
impl PriceLevel {
    pub fn new(price: u64) -> Self {
        Self {
            price,
            orders: VecDeque::new(),
            total_volume: 0,
        }
    }
    pub fn add_order(&mut self, order: Order) {
        self.total_volume += order.quantity;
        self.orders.push_back(order);
    }
}

// Represents the event coming from Kafka
pub enum EngineCommand {
    PlaceOrder(Order),
    CancelOrder(String), // Order ID
    Shutdown,
}

// 2. The Order Book State (Owned solely by the Engine Thread)
struct OrderBook {
    // In a real implementation, bids/asks would be BTreeMaps linking to DoublyLinkedLists
    // For this boilerplate, we'll represent the global map for O(1) cancellations
    orders: HashMap<String, Order>, 
}

impl OrderBook {
    fn new() -> Self {
        OrderBook {
            orders: HashMap::new(),
        }
    }

    fn process_new_order(&mut self, order: Order) {
        // Pseudo-logic: 
        // 1. Check if it crosses the spread (matches against existing orders)
        // 2. If it matches, execute trade, emit TradeExecuted event
        // 3. If remaining quantity > 0, insert into self.orders and the BTreeMap
        println!("Processing Order: {} for {} shares at {}", order.id, order.quantity, order.price);
        self.orders.insert(order.id.clone(), order);
    }

    fn cancel_order(&mut self, order_id: &str) {
        if self.orders.remove(order_id).is_some() {
            println!("Successfully canceled order: {}", order_id);
            // Also remove from the BTreeMap/LinkedList here
        } else {
            println!("Failed to cancel: Order {} not found", order_id);
        }
    }
}

// 3. The Engine Spawner (The entry point)
pub fn spawn_matching_engine() -> Sender<EngineCommand> {
    // Create the lock-free channel
    let (tx, rx): (Sender<EngineCommand>, Receiver<EngineCommand>) = mpsc::channel();

    // Spawn the dedicated matching thread
    thread::spawn(move || {
        println!("🚀 Matching Engine Thread Started. Pinning to core...");
        
        let mut book = OrderBook::new();

        // The Hot Loop: This runs forever, processing events as fast as possible
        loop {
            // rx.recv() blocks until a message arrives. 
            // In a hyper-optimized HFT engine, you might use crossbeam_channel or spinlocks here.
            match rx.recv() {
                Ok(EngineCommand::PlaceOrder(order)) => {
                    book.process_new_order(order);
                }
                Ok(EngineCommand::CancelOrder(id)) => {
                    book.cancel_order(&id);
                }
                Ok(EngineCommand::Shutdown) => {
                    println!("🛑 Shutting down matching engine safely...");
                    break;
                }
                Err(_) => {
                    println!("Channel disconnected. Engine panic.");
                    break;
                }
            }
        }
    });

    // Return the transmitter so the Kafka consumer / API can send orders to the engine
    tx
}

// 4. How you run it in main.rs
// fn main() {
//     // 1. Boot up the engine in the background
//     let engine_tx = spawn_matching_engine();

//     // 2. Simulate Kafka feeding orders into the engine from a different thread
//     let order = Order {
//         id: "ORDER_123".to_string(),
//         user_id: "USER_999".to_string(),
//         ticker: "AAPL".to_string(),
//         side: Side::Buy,
//         price: 15000, // $150.00
//         quantity: 10,
//     };

//     // Send the order to the lock-free channel
//     engine_tx.send(EngineCommand::PlaceOrder(order)).unwrap();
    
//     // Simulate a cancellation
//     engine_tx.send(EngineCommand::CancelOrder("ORDER_123".to_string())).unwrap();

//     // Shut it down
//     engine_tx.send(EngineCommand::Shutdown).unwrap();
    
//     // Slight sleep just to let the background thread print to console before main exits
//     thread::sleep(std::time::Duration::from_millis(50));
// }