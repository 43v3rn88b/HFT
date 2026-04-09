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

#[derive(Deserialize, Debug)]
pub struct CancelOrderRequest {
    pub order_id: String,
    pub ticker: String,
}

// 1. The new Kafka payload wrapper
#[derive(Deserialize, Debug)]
#[serde(tag = "action")] // Looks for an "action" field in the JSON
pub enum IncomingCommand {
    #[serde(rename = "place")]
    Place(Order),
    
    #[serde(rename = "cancel")]
    Cancel {
        id: String,
        ticker: String,
        user_id: String,
    },
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
    CancelOrder { ticker: String, id: String },//CancelOrder(String),
}