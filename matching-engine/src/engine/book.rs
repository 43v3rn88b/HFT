use crate::engine::types::*; // Import your types
use std::collections::{BTreeMap, HashMap};
use std::collections::HashSet;

pub struct LimitOrderBook {
    pub ticker: String,
    // Asks (Sellers): Sorted lowest to highest.
    pub asks: BTreeMap<u64, PriceLevel>, 
    // Bids (Buyers): Sorted lowest to highest.
    pub bids: BTreeMap<u64, PriceLevel>, 
    // O(1) Lookup table for cancellations: OrderID -> Price
    pub order_index: HashMap<String, u64>, 
    pub canceled_orders: HashSet<String>,
}

// spawn_matching_engine();

impl LimitOrderBook {
    pub fn new(ticker: &str) -> Self {
        Self {
            ticker: ticker.to_string(),
            asks: BTreeMap::new(),
            bids: BTreeMap::new(),
            order_index: HashMap::new(),
            canceled_orders: HashSet::new(),
        }
    }

    pub fn take_snapshot(&self) -> OrderBookSnapshot {
        let mut bids = Vec::new();
        // Bids are highest-price first, so we reverse the BTreeMap
        for (&price, level) in self.bids.iter().rev().take(10) {
            bids.push(SnapshotLevel { price, qty: level.total_qty });
        }

        let mut asks = Vec::new();
        // Asks are lowest-price first, so normal iteration is fine
        for (&price, level) in self.asks.iter().take(10) {
            asks.push(SnapshotLevel { price, qty: level.total_qty });
        }

        OrderBookSnapshot {
            ticker: self.ticker.clone(),
            bids,
            asks,
        }
    }

    pub fn cancel_order(&mut self, order_id: String) {
        self.canceled_orders.insert(order_id);
        println!("🗑️ Order {} added to graveyard.", order_id);
    }

    // Returns true if the order was found and removed
    pub fn cancel_order(&mut self, ticker: &str, order_id: &str) -> bool {
        let book = match self.orderbooks.get_mut(ticker) {
            Some(b) => b,
            None => return false,
        };

        let mut removed = false;

        // Example removal logic (adjust based on your exact BTreeMap/Vec structure)
        // 1. Try removing from Bids
        if book.bids.remove_order(order_id) {
            removed = true;
        } 
        // 2. If not in Bids, try Asks
        else if book.asks.remove_order(order_id) {
            removed = true;
        }

        removed
    }
    
    /// The O(1) Cancellation Method
    pub fn cancel_order(&mut self, order_id: &str, side: Side) -> Result<(), &'static str> {
        // 1. Find the price level in O(1) time
        let price = self.order_index.remove(order_id).ok_or("Order not found")?;

        let book = match side {
            Side::Buy => &mut self.bids,
            Side::Sell => &mut self.asks,
        };

        // 2. Find the price level, remove the order, update volume
        if let Some(level) = book.get_mut(&price) {
            if let Some(pos) = level.orders.iter().position(|o| o.id == order_id) {
                let removed_order = level.orders.remove(pos).unwrap();
                level.total_volume -= removed_order.quantity;

                // 3. Cleanup: If the price level is empty, remove it entirely to keep the tree small
                if level.orders.is_empty() {
                    book.remove(&price);
                }
                return Ok(());
            }
        }
        Err("Order not found in price level")
    }
    pub fn process_order(&mut self, mut new_order: Order) {
        if new_order.side == Side::Buy {
            self.match_buy_order(&mut new_order);
            
            // If there's still quantity left after matching, it rests on the book
            if new_order.quantity > 0 {
                self.order_index.insert(new_order.id.clone(), new_order.price);
                let level = self.bids.entry(new_order.price).or_insert(PriceLevel::new(new_order.price));
                level.add_order(new_order);
            }
        } else {
            // Inverse logic for Sell orders against the bids tree...
            // self.match_sell_order(&mut new_order);
        }
    }

    fn match_buy_order(&mut self, buy_order: &mut Order) {
        // Loop while the buy order has quantity AND there are sellers in the book
        while buy_order.quantity > 0 {
            // Get the lowest asking price (O(1) operation on BTreeMap)
            // We use a separate block to appease the borrow checker
            let (best_ask_price, best_ask_level) = match self.asks.iter_mut().next() {
                Some((&price, level)) => (price, level),
                None => break, // No more sellers
            };

            // If the lowest seller wants more money than the buyer is offering, stop.
            if best_ask_price > buy_order.price {
                break; 
            }

            // Get the oldest order at this price level (Time Priority)
            let resting_sell = best_ask_level.orders.front_mut().unwrap();

            // Calculate the trade size
            let trade_qty = std::cmp::min(buy_order.quantity, resting_sell.quantity);

            // Print the trade execution (In real life: push this to a Kafka 'Trades' topic)
            println!("⚡ TRADE EXECUTED: {} shares of {} at {}", trade_qty, self.ticker, best_ask_price);

            // Adjust quantities
            buy_order.quantity -= trade_qty;
            resting_sell.quantity -= trade_qty;
            best_ask_level.total_volume -= trade_qty;

            // If the seller's order is completely filled, remove it
            if resting_sell.quantity == 0 {
                let filled_order = best_ask_level.orders.pop_front().unwrap();
                self.order_index.remove(&filled_order.id);
            }

            // If the price level is now empty, remove the whole price level from the tree
            if best_ask_level.orders.is_empty() {
                self.asks.remove(&best_ask_price);
            }
        }
    }
    
}
