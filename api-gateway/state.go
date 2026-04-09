package main

import "sync"

// OrderBookManager holds the latest snapshot for each ticker
type OrderBookManager struct {
	mu    sync.RWMutex
	books map[string]OrderBookSnapshot
}

func NewOrderBookManager() *OrderBookManager {
	return &OrderBookManager{
		books: make(map[string]OrderBookSnapshot),
	}
}

// Update updates the in-memory book with the latest Kafka snapshot
func (m *OrderBookManager) Update(snapshot OrderBookSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.books[snapshot.Ticker] = snapshot
}

// GetLatest retrieves the current book for a specific ticker
func (m *OrderBookManager) GetLatest(ticker string) (OrderBookSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	book, exists := m.books[ticker]
	return book, exists
}