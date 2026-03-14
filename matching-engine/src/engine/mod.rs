pub mod types;

use std::collections::{BTreeMap, VecDeque};
use std::sync::mpsc::{channel, Receiver, Sender};
use types::{EngineCommand, Order, Side, Trade};

pub fn spawn_matching_engine(trade_tx: Sender<Trade>) -> Sender<EngineCommand> {
    let (engine_tx, engine_rx) = channel::<EngineCommand>();

    std::thread::spawn(move || {
        let mut buy_book: BTreeMap<u64, VecDeque<Order>> = BTreeMap::new();
        let mut sell_book: BTreeMap<u64, VecDeque<Order>> = BTreeMap::new();

        println!("🚀 Matching Engine Live.");

        while let Ok(command) = engine_rx.recv() {
            if let EngineCommand::PlaceOrder(mut order) = command {
                match order.side {
                    Side::Buy => {
                        match_order(&mut order, &mut sell_book, &trade_tx, true);
                        if order.quantity > 0 {
                            buy_book.entry(order.price).or_default().push_back(order);
                        }
                    }
                    Side::Sell => {
                        match_order(&mut order, &mut buy_book, &trade_tx, false);
                        if order.quantity > 0 {
                            sell_book.entry(order.price).or_default().push_back(order);
                        }
                    }
                }
            }
        }
    });

    engine_tx
}

fn match_order(
    incoming: &mut Order,
    book: &mut BTreeMap<u64, VecDeque<Order>>,
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