import { useState, useEffect } from 'react';

export default function LiveTrades() {
  // State to hold our live stream of trades
  const [trades, setTrades] = useState([]);

  useEffect(() => {
    let ws;
    let reconnectTimeout;
    let reconnectDelay = 1000;
    const maxDelay = 10000;

    const connect = () => {
      ws = new WebSocket('ws://172.30.85.187:8082/ws/trades');

      ws.onopen = () => {
        console.log('🟢 UI Connected to Gateway WebSocket');
        reconnectDelay = 1000; // Reset delay
      };

      ws.onmessage = (event) => {
        try {
          const incomingData = JSON.parse(event.data);
          const newTrades = Array.isArray(incomingData) ? incomingData : [incomingData];
          const validTrades = newTrades.filter(trade => typeof trade.price === 'number');

          if (validTrades.length === 0) return;

          setTrades((prevTrades) => {
            const lastKnownPrice = prevTrades.length > 0 ? prevTrades[0].price : null;
            const processedTrades = validTrades.map((trade) => {
              let tickColor = 'text-zinc-300';
              if (lastKnownPrice !== null) {
                if (trade.price > lastKnownPrice) tickColor = 'text-emerald-400';
                if (trade.price < lastKnownPrice) tickColor = 'text-red-400';
              }
              return { ...trade, tickColor };
            });

            return [...processedTrades, ...prevTrades].slice(0, 50);
          });
        } catch (err) {
          console.error('Failed to parse incoming WebSocket data:', err);
        }
      };

      ws.onerror = (error) => console.error('🔴 WebSocket Error.');
      
      ws.onclose = () => {
        console.log(`⚫ WebSocket Disconnected. Retrying in ${reconnectDelay / 1000}s...`);
        reconnectTimeout = setTimeout(connect, reconnectDelay);
        reconnectDelay = Math.min(reconnectDelay * 2, maxDelay);
      };
    };

    connect();

    return () => {
      clearTimeout(reconnectTimeout);
      if (ws && ws.readyState === 1) ws.close();
    };
  }, []);
  
  return (
   <div className="p-4 border border-zinc-800 rounded-lg shadow-xl bg-zinc-900 w-full max-w-md">
      <h2 className="text-xl text-zinc-100 font-bold mb-4 flex items-center gap-2">
        <span className="relative flex h-3 w-3">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75"></span>
          <span className="relative inline-flex rounded-full h-3 w-3 bg-red-500"></span>
        </span>
        Live Trades (AAPL)
      </h2>
      
      <div className="h-64 overflow-y-auto custom-scrollbar pr-2">
        {trades.length === 0 ? (
          <p className="text-zinc-600 italic text-center mt-10">Waiting for market activity...</p>
        ) : (
          <ul className="space-y-2">
            {trades.map((trade, index) => (
              <li 
                key={index} 
                className="p-3 bg-zinc-800/40 rounded border border-zinc-800/50 flex justify-between items-center shadow-sm animate-fade-in-down"
              >
                <span className="font-bold text-zinc-400">{trade.ticker}</span>
               {/* 🚨 This span now dynamically uses the tickColor we calculated */}
                <span className={`font-mono font-bold ${trade.tickColor}`}>
                  ${(trade.price / 100).toFixed(2)}
                </span>
                
                {/* 🚨 THE FIX: Formatted string, right-aligned, fixed minimum width */}
                <span className="font-mono text-xs text-zinc-500 text-right min-w-[70px]">
                  Qty: {Number(trade.quantity).toLocaleString('en-US')}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}