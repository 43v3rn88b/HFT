pub mod types;

use std::collections::{BTreeMap, HashSet, VecDeque};
use std::sync::mpsc::{channel, Receiver, Sender};
use types::{EngineCommand, Order, Side, Trade};
use serde::Serialize;


// --- JSON Snapshot Structs ---
#[derive(Serialize)]
struct SnapshotLevel {
    price: u64,
    qty: u64,
}

#[derive(Serialize)]
struct OrderBookSnapshot {
    ticker: String,
    bids: Vec<SnapshotLevel>,
    asks: Vec<SnapshotLevel>,
}

// --- The Core Engine Thread ---
pub fn spawn_matching_engine(trade_tx: Sender<Trade>, snap_tx: Sender<String>) -> Sender<EngineCommand> {
    let (engine_tx, engine_rx) = channel::<EngineCommand>();

    std::thread::spawn(move || {
        let mut buy_book: BTreeMap<u64, VecDeque<Order>> = BTreeMap::new();
        let mut sell_book: BTreeMap<u64, VecDeque<Order>> = BTreeMap::new();

        let mut canceled_orders: HashSet<String> = HashSet::new();

        println!("🚀 Matching Engine Live.");

        while let Ok(command) = engine_rx.recv() {
            match command {
                EngineCommand::PlaceOrder(mut order) => {
                    
                    let ticker = order.ticker.clone(); 

                    match order.side {
                        Side::Buy => {
                            match_order(&mut order, &mut sell_book, &mut canceled_orders, &trade_tx, true);
                            if order.quantity > 0 {
                                buy_book.entry(order.price).or_default().push_back(order);
                            }
                        }
                        Side::Sell => {
                            match_order(&mut order, &mut buy_book, &mut canceled_orders, &trade_tx, false);
                            if order.quantity > 0 {
                                sell_book.entry(order.price).or_default().push_back(order);
                            }
                        }
                    }

                    send_snapshot(&ticker, &buy_book, &sell_book, &snap_tx);
                }
                EngineCommand::CancelOrder { ticker, id } => {
                    println!("🗑️ Engine is erasing order: {}", id);
                    
                    // 1. Add to your existing lazy cancel list
                    canceled_orders.insert(id.clone());
                    
                    // 2. Actively remove it from the books so the UI updates immediately
                    let mut removed = false;
                    
                    // Check the Buy Book
                    for queue in buy_book.values_mut() {
                        if let Some(pos) = queue.iter().position(|o| o.id == id) {
                            queue.remove(pos);
                            removed = true;
                            break;
                        }
                    }
                    
                    // Check the Sell Book if not found in Bids
                    if !removed {
                        for queue in sell_book.values_mut() {
                            if let Some(pos) = queue.iter().position(|o| o.id == id) {
                                queue.remove(pos);
                                removed = true;
                                break;
                            }
                        }
                    }

                    // 3. Blast out the updated snapshot using your local variables
                    send_snapshot(&ticker, &buy_book, &sell_book, &snap_tx);
                    println!("✅ Order {} removed from book and snapshot sent.", id);
                }
            }
        }
    });

    engine_tx
}

// --- Snapshot Generator ---
fn send_snapshot(
    ticker: &str,
    buy_book: &BTreeMap<u64, VecDeque<Order>>,
    sell_book: &BTreeMap<u64, VecDeque<Order>>,
    snap_tx: &Sender<String>,
) {
    let bids: Vec<SnapshotLevel> = buy_book.iter().rev().take(5).map(|(&price, queue)| {
        SnapshotLevel { price, qty: queue.iter().map(|o| o.quantity).sum() }
    }).collect();

    let asks: Vec<SnapshotLevel> = sell_book.iter().take(5).map(|(&price, queue)| {
        SnapshotLevel { price, qty: queue.iter().map(|o| o.quantity).sum() }
    }).collect();

    let snapshot = OrderBookSnapshot {
        ticker: ticker.to_string(),
        bids,
        asks,
    };

    if let Ok(json_string) = serde_json::to_string(&snapshot) {
        let _ = snap_tx.send(json_string);
    }
}

// --- The Matching Algorithm ---
fn match_order(
    incoming: &mut Order,
    book: &mut BTreeMap<u64, VecDeque<Order>>,
    canceled_orders: &mut HashSet<String>,
    trade_tx: &Sender<Trade>,
    is_buy_order: bool,
) {
    loop {
        let best_price = if is_buy_order {
            book.keys().next().cloned() 
        } else {
            book.keys().next_back().cloned() 
        };

        match best_price {
            Some(price) if (is_buy_order && incoming.price >= price) || (!is_buy_order && incoming.price <= price) => {
                if let Some(orders_at_price) = book.get_mut(&price) {
                    if let Some(mut existing_order) = orders_at_price.pop_front() {
                        // --- THE LAZY CANCEL CHECK ---
                        if canceled_orders.contains(&existing_order.id) {
                            // 1. Remove it from the graveyard to save memory
                            canceled_orders.remove(&existing_order.id);
                            
                            // 2. Clean up the price level if it's now empty
                            if orders_at_price.is_empty() {
                                book.remove(&price);
                            }
                            
                            // 3. Skip the trade! Jump to the next iteration of the loop.
                            continue; 
                        }
                        let fill_qty = std::cmp::min(incoming.quantity, existing_order.quantity);
                        
                        let trade = Trade {
                            buy_order_id: if is_buy_order { incoming.id.clone() } else { existing_order.id.clone() },
                            sell_order_id: if is_buy_order { existing_order.id.clone() } else { incoming.id.clone() },
                            ticker: incoming.ticker.clone(),
                            price,
                            quantity: fill_qty,
                        };

                        trade_tx.send(trade).unwrap();

                        incoming.quantity -= fill_qty;
                        existing_order.quantity -= fill_qty;

                        if existing_order.quantity > 0 {
                            orders_at_price.push_front(existing_order);
                        }
                    }

                    if orders_at_price.is_empty() {
                        book.remove(&price);
                    }
                }
                
                if incoming.quantity == 0 { break; }
            }
            _ => break, 
        }
    }
}