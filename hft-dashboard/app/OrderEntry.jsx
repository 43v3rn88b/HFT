import { useState } from 'react';

export default function OrderEntry() {
  const [side, setSide] = useState('Buy');
  const [price, setPrice] = useState('');
  const [quantity, setQuantity] = useState('');
  const [status, setStatus] = useState(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setIsSubmitting(true);
    setStatus(null);

    // Convert dollar input to integer cents for the Go backend
    const priceInCents = Math.round(parseFloat(price) * 100);

    const orderPayload = {
      id: `ui_${Date.now()}`, // Generate a quick unique ID
      user_id: "user123",     // Hardcoded for this test phase
      ticker: "AAPL",
      side: side,
      price: priceInCents,
      quantity: parseInt(quantity, 10),
    };

    try {
      const response = await fetch('http://172.30.85.187:8082/orders', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(orderPayload),
      });

      const data = await response.json();

      if (!response.ok) {
        throw new Error(data.error || 'Order rejected by risk engine');
      }

      setStatus({ type: 'success', message: `Order Sent: ${side} ${quantity} AAPL @ $${price}` });
      
      // Optional: Clear the form after a successful order
      // setPrice('');
      // setQuantity('');
      
    } catch (error) {
      setStatus({ type: 'error', message: error.message });
    } finally {
      setIsSubmitting(false);
      
      // Clear the status message after 3 seconds
      setTimeout(() => setStatus(null), 3000);
    }
  };

  return (
    <div className="p-4 border border-zinc-800 rounded-lg shadow-xl bg-zinc-900 w-full max-w-md">
    <h2 className="text-xl font-bold mb-4 border-b border-zinc-800 pb-2 text-zinc-100">Place Order 
      <span className="text-zinc-500 text-sm font-normal">AAPL</span>
      </h2>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        {/* Buy / Sell Toggle */}
        <div className="flex gap-4">
          <button
            type="button"
            onClick={() => setSide('Buy')}
            className={`flex-1 py-2 font-bold rounded transition-colors ${
              side === 'Buy' ? 'bg-green-500 text-white' : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
            }`}
          >
            BUY
          </button>
          <button
            type="button"
            onClick={() => setSide('Sell')}
            className={`flex-1 py-2 font-bold rounded transition-colors ${
              side === 'Sell' ? 'bg-red-500 text-white' : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
            }`}
          >
            SELL
          </button>
        </div>

        {/* Price and Quantity Inputs */}
        <div className="flex gap-4">
          <div className="flex-1">
            <label className="block text-xs font-mono text-zinc-500 mb-1">Price (USD)</label>
            <input
              type="number"
              step="0.01"
              min="0.01"
              required
              value={price}
              onChange={(e) => setPrice(e.target.value)}
              className="w-full p-2 bg-zinc-800 border border-zinc-700 rounded text-zinc-100 focus:ring-1 focus:ring-emerald-500 outline-none font-mono"
              placeholder="150.00"
            />
          </div>
          <div className="flex-1">
            <label className="block text-xs font-mono text-zinc-500 mb-1">Quantity</label>
            <input
              type="number"
              min="1"
              required
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
              className="w-full p-2 bg-zinc-800 border border-zinc-700 rounded text-zinc-100 focus:ring-1 focus:ring-emerald-500 outline-none font-mono"
              placeholder="10"
            />
          </div>
        </div>

        {/* Submit Button */}
        <button
          type="submit"
          disabled={isSubmitting}
          className={`w-full py-3 font-bold rounded text-white mt-2 transition-colors ${
            isSubmitting ? 'bg-gray-400 cursor-not-allowed' : 'bg-blue-600 hover:bg-blue-700'
          }`}
        >
          {isSubmitting ? 'Processing...' : 'Submit Order'}
        </button>
      </form>

      {/* Status Feedback Message */}
      {status && (
        <div className={`mt-4 p-3 rounded text-sm font-medium ${
          status.type === 'error' ? 'bg-red-100 text-red-700' : 'bg-green-100 text-green-700'
        }`}>
          {status.message}
        </div>
      )}
    </div>
  );
}