'use client';
import { useEffect, useState, useRef } from 'react';

export default function TradeTicker() {
  const [trades, setTrades] = useState([]);

  // 1. Smart Scroll States
  const [isHovered, setIsHovered] = useState(false);
  const [isScrolled, setIsScrolled] = useState(false);
  const [queuedCount, setQueuedCount] = useState(0);

  const bufferRef = useRef([]);
  const scrollContainerRef = useRef(null);

  // Derived pause state
  const isPaused = isHovered || isScrolled;
  const isPausedRef = useRef(isPaused);

  // Keep the ref strictly in sync with the state so the WebSocket closure can read it
  useEffect(() => {
    isPausedRef.current = isPaused;

    // If we just UNPAUSED, flush the buffered trades into the view
    if (!isPaused && bufferRef.current.length > 0) {
      setTrades((prevTrades) => {
        const merged = [...bufferRef.current, ...prevTrades].slice(0, 100);
        bufferRef.current = [];
        setQueuedCount(0);
        return merged;
      });
    }
  }, [isPaused]);

  useEffect(() => {
    let ws;
    let reconnectTimeout;
    let reconnectDelay = 1000;
    const maxDelay = 10000;

    const connect = () => {
      ws = new WebSocket('ws://172.30.85.187:8082/ws/trades');

      ws.onopen = () => {
        console.log('🟢 UI Connected to Ticker Stream');
        reconnectDelay = 1000;
      };
    ws.onmessage = (event) => {
      try {
        // 1. Parse the incoming BATCH (Array) from Go
        const incomingBatch = JSON.parse(event.data);
        
        // 2. Format every trade in the batch
        const formattedBatch = incomingBatch.map((incomingTrade, index) => ({
          id: incomingTrade.buy_order_id + incomingTrade.sell_order_id + index, 
          price: (incomingTrade.price / 100).toFixed(2),
          quantity: incomingTrade.quantity || 1,
          time: new Date().toLocaleTimeString('en-US', { 
            hour12: false, 
            hour: '2-digit', 
            minute: '2-digit', 
            second: '2-digit', 
            fractionalSecondDigits: 3 
          })
        }));

        // 2. The Smart Scroll Routing Logic
          if (isPausedRef.current) {
            // If paused, shove them in the hidden buffer and update the counter
            bufferRef.current = [...formattedBatch, ...bufferRef.current].slice(0, 100);
            setQueuedCount(bufferRef.current.length);
          } else {
            // If live, render them immediately
            setTrades((prevTrades) => [...formattedBatch, ...prevTrades].slice(0, 100));
          }

      } catch (err) {
        console.error('Ticker parsing error:', err);
      }
    };
    ws.onerror = () => console.error('🔴 Ticker WS Error');

      ws.onclose = () => {
        console.log(`⚫ Ticker WS Disconnected. Retrying in ${reconnectDelay / 1000}s...`);
        reconnectTimeout = setTimeout(connect, reconnectDelay);
        reconnectDelay = Math.min(reconnectDelay * 2, maxDelay);
      };
    };

    connect();

    return () => {
      clearTimeout(reconnectTimeout);
      if (ws.readyState === 1) ws.close();
    };
  }, []);

  // 3. Handlers for Mouse and Scroll events
  const handleScroll = () => {
    if (!scrollContainerRef.current) return;
    // If the user scrolls down more than 10 pixels, freeze the feed
    setIsScrolled(scrollContainerRef.current.scrollTop > 10);
  };

  const handleResume = () => {
    if (scrollContainerRef.current) {
      // Smoothly scroll back to the top
      scrollContainerRef.current.scrollTo({ top: 0, behavior: 'smooth' });
    }
    setIsHovered(false); // Force un-hover if they clicked the button
  };

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4 flex flex-col h-full shadow-xl">
      {/* Header */}
      <div className="flex justify-between items-center mb-3 border-b border-zinc-800 pb-2">
      <h3 className="text-zinc-400 font-bold mb-3 border-b border-zinc-800 pb-2 text-sm tracking-wider">
        TIME & SALES
      </h3>
      
      {/* 4. The Live / Paused Indicator */}
        {isPaused ? (
           <button
              onClick={handleResume}
              className="text-[10px] font-bold bg-amber-500/20 text-amber-400 px-2 py-1 rounded border border-amber-500/30 flex items-center gap-1 hover:bg-amber-500/30 transition-colors cursor-pointer"
           >
              <span className="animate-pulse">⏸ PAUSED ({queuedCount})</span> 
              <span>🔼</span>
           </button>
        ) : (
           <div className="flex items-center gap-2">
             <span className="relative flex h-1.5 w-1.5">
               <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
               <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-emerald-500"></span>
             </span>
             <span className="text-[10px] text-zinc-500 font-mono">LIVE</span>
           </div>
        )}
      </div>
      
      <div className="grid grid-cols-3 text-xs text-zinc-500 font-mono mb-2 px-2">
        <span>TIME</span>
        <span className="text-right">PRICE</span>
        <span className="text-right">QTY</span>
      </div>

        {/* 5. The Scroll Container with Mouse and Scroll Tracking */}
      <div
        ref={scrollContainerRef}
          onScroll={handleScroll}
          onMouseEnter={() => setIsHovered(true)}
          onMouseLeave={() => setIsHovered(false)} 
        className="overflow-y-auto flex-grow pr-1 custom-scrollbar">
        {trades.length === 0 ? (
          <div className="text-zinc-600 text-sm text-center mt-4 italic">Waiting for trades...</div>
        ) : (
          <div className="flex flex-col gap-1">
            {trades.map((trade) => (
              <div 
                key={trade.id} 
                className="grid grid-cols-3 text-sm font-mono px-2 py-1 rounded bg-zinc-800/30 hover:bg-zinc-800 transition-colors animate-fade-in-down"
              >
                <span className="text-zinc-400">{trade.time}</span>
                {/* 🚨 THE FIX: Right-aligned price */}
                <span className="text-zinc-300 font-bold text-right">${trade.price}</span>
                {/* 🚨 THE FIX: Right-aligned and formatted quantity */}
                <span className="text-zinc-500 text-right">{Number(trade.quantity).toLocaleString('en-US')}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}