"use client";

import { useEffect, useState } from "react";
import OrderForm from "./OrderForm";
import LiveTrades from "./LiveTrades";
import OrderBook from "./OrderBook";
import OrderEntry from './OrderEntry';
import ActiveOrders from './ActiveOrders';
import PriceChart from './PriceChart';
import TradeTicker from './TradeTicker';

export default function Dashboard() {
  return (
    <div className="min-h-screen bg-black p-6 text-white font-sans">
      <div className="max-w-[1600px] mx-auto flex flex-col gap-6">
        
        {/* Header */}
        <div className="flex justify-between items-center border-b border-zinc-800 pb-4">
          <h1 className="text-2xl font-black tracking-tighter italic">HFT_TERMINAL_v1.0</h1>
          <div className="flex gap-4 items-center">
            <span className="text-xs bg-emerald-500/10 text-emerald-500 px-3 py-1 rounded-full border border-emerald-500/20">
              SYSTEM_LIVE
            </span>
          </div>
        </div>

        {/* Top Section: Chart (Main Focus) & Order Book */}
        <div className="flex flex-col lg:flex-row gap-6">
          <div className="flex-1 min-h-[450px]">
             <PriceChart />
             <TradeTicker />
          </div>
          <div className="w-full lg:w-[350px]">
             {/* 🚨 Notice how we just call OrderBook without any props now! */}
             <OrderBook />
          </div>
        </div>

        {/* Middle Section: Execution Tools */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          <OrderEntry />
          <LiveTrades />
          <div className="bg-zinc-900/50 border border-zinc-800 rounded-lg p-4">
             {/* Placeholder for future Portfolio or Stats component */}
             <h3 className="text-zinc-500 text-xs font-bold uppercase mb-4">Account Stats</h3>
             <div className="text-zinc-600 italic text-sm">
                Integration pending...
             </div>
          </div>
        </div>

      </div>
    </div>
  );
}