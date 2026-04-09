package main

import (
	// "encoding/json"
	// "log"
	// "sync"
)

// PriceLevel represents a single row on the order book ladder
type PriceLevel struct {
	Price uint64 `json:"price"`
	Qty   uint64 `json:"qty"`
}

// OrderBookSnapshot matches the exact JSON emitted by the Rust engine
type OrderBookSnapshot struct {
	Ticker string       `json:"ticker"`
	Bids   []PriceLevel `json:"bids"`
	Asks   []PriceLevel `json:"asks"`
}