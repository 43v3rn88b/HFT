'use client';
import { useState, useEffect, useMemo } from 'react';

// Define the exact shape of the data coming from Rust
type PriceLevel = { price: number; qty: number };
type OrderBookSnapshot = { ticker: string; bids: PriceLevel[]; asks: PriceLevel[] };

export default function OrderBook() {
  // 1. Hook: useState
  const [orderBook, setOrderBook] = useState<OrderBookSnapshot | null>(null);

  // 3. Connect to the Order Book WebSocket with AUTO-RECONNECT
  useEffect(() => {
    let ws: WebSocket;
    let reconnectTimeout: NodeJS.Timeout;
    let reconnectDelay = 1000; // Start with a 1-second delay
    const maxDelay = 10000;    // Cap the maximum delay at 10 seconds

    const connect = () => {
      console.log('⏳ Attempting to connect to Order Book WS...');
      ws = new WebSocket('ws://172.30.85.187:8082/ws/orderbook');
      
      ws.onopen = () => {
        console.log('🟢 UI Connected to Order Book Stream');
        reconnectDelay = 1000; // Reset the delay back to 1s on a successful connection
      };
      
      ws.onmessage = (event) => {
        try {
          const snapshot = JSON.parse(event.data);
          setOrderBook(snapshot);
        } catch (err) {
          console.error('Failed to parse order book snapshot:', err);
        }
      };

      ws.onerror = (error) => {
        console.error('🔴 Order Book WS Error. Connection failed.');
        // We don't reconnect here. We let it naturally fall through to onclose!
      };

      ws.onclose = () => {
        console.log(`⚫ Order Book WS Disconnected. Retrying in ${reconnectDelay / 1000}s...`);
        // Schedule the next connection attempt
        reconnectTimeout = setTimeout(connect, reconnectDelay);
        // Multiply the delay by 2 for next time (Exponential Backoff)
        reconnectDelay = Math.min(reconnectDelay * 2, maxDelay);
      };
    };

    // Kick off the first connection
    connect();

    // Cleanup function when the component unmounts
    return () => {
      clearTimeout(reconnectTimeout); // Stop the retry loop if the user leaves the page
      if (ws && ws.readyState === 1) ws.close();
    };
  }, []);

  const formatPrice = (cents: number) => (cents / 100).toFixed(2);

  // Safely extract arrays (even if orderBook is null, they default to empty arrays)
  const bids = orderBook?.bids || [];
  const asks = orderBook?.asks || [];

  // 3. Hook: useMemo (Now safely executing on EVERY render)
  const { depthBids, depthAsks, maxDepth } = useMemo(() => {
    let currentBidTotal = 0;
    const dBids = bids.map(b => {
      currentBidTotal += b.qty;
      return { ...b, cumulative: currentBidTotal };
    });

    let currentAskTotal = 0;
    const dAsks = [...asks].map(a => {
      currentAskTotal += a.qty;
      return { ...a, cumulative: currentAskTotal };
    });

    const max = Math.max(currentBidTotal, currentAskTotal, 1);
    return { depthBids: dBids, depthAsks: dAsks, maxDepth: max };
  }, [bids, asks]);

  // SPREAD CALCULATION
  const highestBid = bids.length > 0 ? bids[0].price : 0;
  const lowestAsk = asks.length > 0 ? asks[0].price : 0;
  
  const spreadCents = (lowestAsk > 0 && highestBid > 0) ? (lowestAsk - highestBid) : 0;
  const spreadPercentage = highestBid > 0 ? ((spreadCents / highestBid) * 100).toFixed(2) : "0.00";

  // 🚨 EARLY RETURN GOES HERE 🚨
  // Now that all hooks have safely run, we can stop the render if there's no data.
  if (!orderBook) {
    return (
      <div className="p-4 border border-zinc-800 rounded-lg text-zinc-500 bg-zinc-900/50 flex items-center justify-center h-full min-h-[300px]">
        <span className="animate-pulse">Waiting for market data...</span>
      </div>
    );
  }

  // --- THE MAIN UI RENDER ---
  return (
    <div className="bg-zinc-950 border border-zinc-800 rounded-lg flex flex-col h-full shadow-2xl overflow-hidden min-h-[400px]">
      {/* Header */}
      <div className="flex justify-between items-center p-3 border-b border-zinc-800 bg-zinc-900/50">
        <h3 className="text-xs font-bold text-zinc-400 tracking-widest uppercase">Order Book</h3>
        <span className="text-[10px] text-zinc-500 font-mono">{orderBook.ticker || "AAPL"}</span>
      </div>

      {/* Columns Header */}
      <div className="grid grid-cols-2 px-4 py-2 text-[10px] font-bold text-zinc-600 border-b border-zinc-900">
        <span>SIZE</span>
        <span className="text-right">PRICE (USD)</span>
      </div>

      <div className="flex-grow overflow-y-auto custom-scrollbar font-mono">
        {/* ASKS (Sellers) */}
        <div className="flex flex-col-reverse gap-[1px]">
          {depthAsks.length === 0 ? (
            <p className="text-zinc-700 text-center text-xs py-4 italic">No sellers</p>
          ) : (
            depthAsks.map((ask) => (
              <div key={`ask-${ask.price}`} className="relative flex justify-between px-4 py-1 text-xs group hover:bg-red-500/5 transition-colors">
                <div 
                  className="absolute right-0 top-0 h-full bg-red-500/10 transition-all duration-300"
                  style={{ width: `${(ask.cumulative / maxDepth) * 100}%` }}
                />
                <span className="relative z-10 text-zinc-400 text-left w-1/2">{ask.qty.toLocaleString('en-US')}</span>
                <span className="relative z-10 text-red-500 font-bold text-right w-1/2">{formatPrice(ask.price)}</span>
              </div>
            ))
          )}
        </div>

        {/* THE SPREAD DIVIDER */}
        <div className="flex flex-col justify-center items-center py-3 bg-zinc-900/80 border-y border-zinc-800 my-1">
          <span className="text-[9px] text-zinc-500 font-bold tracking-[0.2em] mb-1">SPREAD</span>
          {spreadCents > 0 ? (
             <div className="flex items-center gap-2">
               <span className="text-emerald-400 font-mono font-semibold">${(spreadCents / 100).toFixed(2)}</span>
               <span className="text-zinc-500 font-mono text-[10px]">({spreadPercentage}%)</span>
             </div>
           ) : (
             <span className="text-zinc-600 font-mono text-sm">---</span>
           )}
        </div>

        {/* BIDS (Buyers) */}
        <div className="flex flex-col gap-[1px]">
          {depthBids.length === 0 ? (
            <p className="text-zinc-700 text-center text-xs py-4 italic">No buyers</p>
          ) : (
            depthBids.map((bid) => (
              <div key={`bid-${bid.price}`} className="relative flex justify-between px-4 py-1 text-xs group hover:bg-emerald-500/5 transition-colors">
                <div 
                  className="absolute right-0 top-0 h-full bg-emerald-500/10 transition-all duration-300"
                  style={{ width: `${(bid.cumulative / maxDepth) * 100}%` }}
                />
                <span className="relative z-10 text-zinc-400 text-left w-1/2">{bid.qty.toLocaleString('en-US')}</span>
                <span className="relative z-10 text-emerald-500 font-bold text-right w-1/2">{formatPrice(bid.price)}</span>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}