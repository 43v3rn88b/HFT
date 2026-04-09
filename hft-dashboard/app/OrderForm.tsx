"use client";

import { useState } from "react";

export default function OrderForm() {
  const [side, setSide] = useState<"Buy" | "Sell">("Buy");
  const [price, setPrice] = useState("151.00");
  const [quantity, setQuantity] = useState("10");
  const [status, setStatus] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setStatus("Sending...");

    const orderPayload = {
      // Generate a random ID for this trade
      id: `trade-${Math.random().toString(36).substring(2, 9)}`, 
      user_id: "user123", // Hardcoded for this demo
      ticker: "AAPL",
      side: side,
      // Convert standard dollars string back to integer cents for the backend
      price: Math.round(parseFloat(price) * 100), 
      quantity: parseInt(quantity),
    };

    try {
      // Make sure this IP matches your WSL IP!
      const response = await fetch("http://172.30.85.187:8082/orders", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(orderPayload),
      });

      if (response.ok) {
        setStatus(`✅ ${side} order placed!`);
        setTimeout(() => setStatus(""), 2000); // Clear success message after 2s
      } else {
        const errData = await response.json();
        setStatus(`❌ Error: ${errData.error || "Failed"}`);
      }
    } catch (error) {
      console.error(error);
      setStatus("❌ Network error connecting to Gateway");
    }
  };

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg shadow-2xl p-6 flex flex-col h-full">
      <h2 className="text-xl font-bold tracking-wider mb-6 border-b border-zinc-800 pb-2">
        EXECUTE TRADE
      </h2>

      <form onSubmit={handleSubmit} className="flex flex-col space-y-4 flex-grow">
        
        {/* Buy / Sell Toggle */}
        <div className="grid grid-cols-2 gap-2 bg-zinc-950 p-1 rounded-md">
          <button
            type="button"
            onClick={() => setSide("Buy")}
            className={`py-2 rounded font-bold transition-all ${
              side === "Buy" ? "bg-emerald-600 text-white" : "text-zinc-500 hover:text-zinc-300"
            }`}
          >
            BUY
          </button>
          <button
            type="button"
            onClick={() => setSide("Sell")}
            className={`py-2 rounded font-bold transition-all ${
              side === "Sell" ? "bg-red-600 text-white" : "text-zinc-500 hover:text-zinc-300"
            }`}
          >
            SELL
          </button>
        </div>

        {/* Price Input */}
        <div>
          <label className="text-xs text-zinc-500 font-semibold mb-1 block">PRICE ($)</label>
          <input
            type="number"
            step="0.01"
            value={price}
            onChange={(e) => setPrice(e.target.value)}
            className="w-full bg-zinc-950 border border-zinc-800 rounded p-3 text-white focus:outline-none focus:border-zinc-500 transition-colors"
            required
          />
        </div>

        {/* Quantity Input */}
        <div>
          <label className="text-xs text-zinc-500 font-semibold mb-1 block">QUANTITY</label>
          <input
            type="number"
            value={quantity}
            onChange={(e) => setQuantity(e.target.value)}
            className="w-full bg-zinc-950 border border-zinc-800 rounded p-3 text-white focus:outline-none focus:border-zinc-500 transition-colors"
            required
          />
        </div>

        {/* Spacer to push button to bottom */}
        <div className="flex-grow"></div>

        {/* Submit Button */}
        <button
          type="submit"
          className={`w-full py-4 rounded font-bold text-lg transition-all ${
            side === "Buy" ? "bg-emerald-600 hover:bg-emerald-500" : "bg-red-600 hover:bg-red-500"
          }`}
        >
          {side} AAPL
        </button>

        {/* Status Message */}
        <div className="h-6 text-center text-sm font-medium">
          {status}
        </div>
      </form>
    </div>
  );
}