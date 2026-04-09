import { useState } from 'react';

export default function ActiveOrders() {
  // Simulating the user's resting orders for the UI
  // In a full app, you'd fetch these from your Go backend or a Redis cache
  const [myOrders, setMyOrders] = useState([
    { id: 'order_buy_102', ticker: 'AAPL', side: 'Buy', price: 14500, quantity: 50 },
    { id: 'order_sell_103', ticker: 'AAPL', side: 'Sell', price: 16500, quantity: 25 },
  ]);

  const [cancellingId, setCancellingId] = useState(null);

  const handleCancel = async (ticker, orderId) => {
    setCancellingId(orderId);

    try {
      // Hitting your Gin DELETE route using the WSL IP
      const response = await fetch(`http://172.30.85.187:8082/orders/${ticker}/${orderId}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to cancel order');
      }

      console.log(`🗑️ Successfully sent cancel command for ${orderId}`);
      
      // Remove the order from the UI immediately
      setMyOrders((prev) => prev.filter((order) => order.id !== orderId));

    } catch (error) {
      console.error('Cancel Error:', error);
      alert(`Could not cancel order: ${error.message}`);
    } finally {
      setCancellingId(null);
    }
  };

  return (
    <div className="p-4 border rounded-lg shadow-md bg-white w-full max-w-3xl">
      <h2 className="text-xl font-bold mb-4 border-b pb-2 text-black" >My Active Orders</h2>
      
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="bg-gray-100 text-gray-600 text-sm uppercase">
              <th className="p-3 border-b">Order ID</th>
              <th className="p-3 border-b">Ticker</th>
              <th className="p-3 border-b">Side</th>
              <th className="p-3 border-b">Price</th>
              <th className="p-3 border-b">Qty</th>
              <th className="p-3 border-b text-right">Action</th>
            </tr>
          </thead>
          <tbody>
            {myOrders.length === 0 ? (
              <tr>
                <td colSpan="6" className="p-4 text-center text-gray-500 italic">
                  No active orders
                </td>
              </tr>
            ) : (
              myOrders.map((order) => (
                <tr key={order.id} className="border-b hover:bg-gray-50 transition-colors font-mono text-sm">
                  <td className="p-3 text-gray-500">{order.id}</td>
                  <td className="p-3 font-bold text-gray-500">{order.ticker}</td>
                  <td className={`p-3 font-bold ${order.side === 'Buy' ? 'text-green-600' : 'text-red-600'}`}>
                    {order.side}
                  </td>
                  <td className="p-3 text-gray-500">${(order.price / 100).toFixed(2)}</td>
                  <td className="p-3 text-gray-500">{order.quantity}</td>
                  <td className="p-3 text-right">
                    <button
                      onClick={() => handleCancel(order.ticker, order.id)}
                      disabled={cancellingId === order.id}
                      className={`px-3 py-1 rounded font-bold text-xs transition-colors ${
                        cancellingId === order.id
                          ? 'bg-gray-300 text-gray-500 cursor-not-allowed'
                          : 'bg-red-100 text-red-700 hover:bg-red-200 border border-red-200'
                      }`}
                    >
                      {cancellingId === order.id ? 'Cancelling...' : 'Cancel'}
                    </button>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}