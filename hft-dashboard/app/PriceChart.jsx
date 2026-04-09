'use client';
import { useEffect, useRef } from 'react';
import { createChart } from 'lightweight-charts';

export default function PriceChart() {
  const chartContainerRef = useRef();
  const chartInstance = useRef();
  const candleSeries = useRef();
  const volumeSeries = useRef();
  
  const currentCandle = useRef(null);
  const currentVolume = useRef(null);
  
  // 👈 NEW: Refs for our high-performance UI updates
  const priceDisplayRef = useRef(null);
  const lastPriceRef = useRef(0);

  useEffect(() => {
    if (!chartContainerRef.current) return;
    
    // 1. Initialize the Chart
    const chart = createChart(chartContainerRef.current, {
      layout: {
        background: { type: 'solid', color: '#0b0e11' },
        textColor: '#d1d4dc',
      },
      grid: {
        vertLines: { color: '#2B2B43' },
        horzLines: { color: '#2B2B43' },
      },
      width: chartContainerRef.current.clientWidth,
      height: 400,
      timeScale: {
        timeVisible: true,
        secondsVisible: false,
      },
    });

    // 2. Add Candlestick Series
    const cSeries = chart.addCandlestickSeries({
      upColor: '#26a69a',
      downColor: '#ef5350',
      borderVisible: false,
      wickUpColor: '#26a69a',
      wickDownColor: '#ef5350',
    });

    // 3. Add Volume Series
    const vSeries = chart.addHistogramSeries({
      color: '#26a69a',
      priceFormat: { type: 'volume' },
      priceScaleId: '', 
    });

    vSeries.priceScale().applyOptions({
      scaleMargins: { top: 0.8, bottom: 0 },
    });

    chartInstance.current = chart;
    candleSeries.current = cSeries;
    volumeSeries.current = vSeries;

    const handleResize = () => {
      chart.applyOptions({ width: chartContainerRef.current.clientWidth });
    };
    window.addEventListener('resize', handleResize);

    // 2. WebSocket Auto-Reconnect Logic
    let ws;
    let reconnectTimeout;
    let reconnectDelay = 1000;
    const maxDelay = 10000;

    const connect = () => {
      ws = new WebSocket('ws://172.30.85.187:8082/ws/trades');

      ws.onopen = () => {
        console.log('🟢 UI Connected to Chart Stream');
        reconnectDelay = 1000; // Reset the delay on success
      };
    ws.onmessage = (event) => {
      try {
        const incomingData = JSON.parse(event.data);
        
        // 🚨 NEW: Force it into an array
        const tradesArray = Array.isArray(incomingData) ? incomingData : [incomingData];

        // Process every trade in the batch one by one
        tradesArray.forEach(trade => {
          // 🚨 ADD THIS LINE: If there's no price, ignore the message!
          if (typeof trade.price !== 'number') return;
          const price = trade.price / 100; // Convert cents to dollars
          const quantity = trade.quantity || trade.Qty || 1;
          
          // --- 🟢 NEW: DIRECT DOM MANIPULATION FOR LOW LATENCY UI 🔴 ---
          if (priceDisplayRef.current) {
            // Format price to 2 decimal places
            const formattedPrice = price.toFixed(2);
            priceDisplayRef.current.innerText = price.toFixed(2);

            // Compare with last price to determine glow color
            if (lastPriceRef.current !== 0) {
              if (price > lastPriceRef.current) {
                // UP TICK: Glowing Green
                priceDisplayRef.current.className = "text-2xl font-mono font-bold text-emerald-400 drop-shadow-[0_0_8px_rgba(52,211,153,0.8)] transition-colors duration-75";
              } else if (price < lastPriceRef.current) {
                // DOWN TICK: Glowing Red
                priceDisplayRef.current.className = "text-2xl font-mono font-bold text-red-400 drop-shadow-[0_0_8px_rgba(248,113,113,0.8)] transition-colors duration-75";
              } else {
                // NEUTRAL: Fade back to white if price didn't change
                priceDisplayRef.current.className = "text-2xl font-mono font-bold text-zinc-100 transition-colors duration-500";
              }
            }
            lastPriceRef.current = price;
          }
          const time = Math.floor(Date.now() / 60000) * 60; 

          if (!currentCandle.current || currentCandle.current.time !== time) {
            currentCandle.current = { time, open: price, high: price, low: price, close: price };
            currentVolume.current = { time, value: quantity, color: '#26a69a' };
          } else {
            currentCandle.current.high = Math.max(currentCandle.current.high, price);
            currentCandle.current.low = Math.min(currentCandle.current.low, price);
            currentCandle.current.close = price;
            currentVolume.current.value += quantity;
            
            currentVolume.current.color = currentCandle.current.close >= currentCandle.current.open 
              ? 'rgba(38, 166, 154, 0.5)' 
              : 'rgba(239, 83, 80, 0.5)';
          }

          cSeries.update(currentCandle.current);
          vSeries.update(currentVolume.current);

        });
        
      } catch (err) {
        console.error('Chart trade parsing error:', err);
      }
    };
    ws.onerror = () => console.error('🔴 Chart WS Error');

      ws.onclose = () => {
        console.log(`⚫ Chart WS Disconnected. Retrying in ${reconnectDelay / 1000}s...`);
        reconnectTimeout = setTimeout(connect, reconnectDelay);
        reconnectDelay = Math.min(reconnectDelay * 2, maxDelay);
      };
    };

    // 3. Kick off the connection loop
    connect();

    return () => {
      window.removeEventListener('resize', handleResize);
      clearTimeout(reconnectTimeout);
      if (ws && ws.readyState === 1) ws.close();
      chart.remove();
    };
  }, []);

  return (
    <div className="p-4 border border-zinc-800 rounded-lg shadow-xl bg-zinc-900 w-full h-full flex flex-col">
      {/* 👈 NEW HEADER LAYOUT */}
      <div className="flex justify-between items-end mb-4 border-b border-zinc-800 pb-2">
        <div>
          <h2 className="text-xl font-bold text-zinc-100 tracking-wider inline-block mr-3">
            AAPL <span className="text-zinc-500 text-sm font-normal">/ USD</span>
          </h2>
          {/* This is the span our ref directly manipulates */}
          <span ref={priceDisplayRef} className="text-2xl font-mono font-bold text-zinc-500">
            ---
          </span>
        </div>
        
        {/* Optional: Add a subtle live indicator */}
        <div className="flex items-center gap-2 mb-1">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
          </span>
          <span className="text-xs text-zinc-500 font-mono">LIVE</span>
        </div>
      </div>

      <div ref={chartContainerRef} className="w-full flex-grow" />
    </div>
  );
}