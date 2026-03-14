use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Debug, Clone, PartialEq)]
pub enum Side {
    Buy,
    Sell,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct Order {
    pub id: String,
    pub user_id: String,
    pub ticker: String,
    pub side: Side,
    pub price: u64,
    pub quantity: u64,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct Trade {
    pub buy_order_id: String,
    pub sell_order_id: String,
    pub ticker: String,
    pub price: u64,
    pub quantity: u64,
}

pub enum EngineCommand {
    PlaceOrder(Order),
}