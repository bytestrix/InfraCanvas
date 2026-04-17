'use client';

import { CheckCircle, XCircle, Clock } from 'lucide-react';

export default function ActionHistory() {
  // Mock data - in real implementation, this would come from a store or API
  const history = [
    {
      id: '1',
      type: 'Update Image',
      timestamp: '2024-01-15 10:30:00',
      vms: 150,
      success: 148,
      failed: 2,
      status: 'completed',
    },
    {
      id: '2',
      type: 'Scale Deployment',
      timestamp: '2024-01-15 09:15:00',
      vms: 50,
      success: 50,
      failed: 0,
      status: 'completed',
    },
    {
      id: '3',
      type: 'Update Image',
      timestamp: '2024-01-14 16:45:00',
      vms: 69,
      success: 69,
      failed: 0,
      status: 'completed',
    },
  ];

  return (
    <div className="space-y-4">
      <div className="bg-gray-50 border border-gray-200 rounded-lg p-4">
        <h3 className="font-semibold text-gray-900 mb-2">📜 Action History</h3>
        <p className="text-sm text-gray-700">
          View past operations and their results
        </p>
      </div>

      <div className="space-y-3">
        {history.map((item) => (
          <div key={item.id} className="border border-gray-200 rounded-lg p-4 hover:shadow-md transition-shadow">
            <div className="flex items-start justify-between">
              <div className="flex-1">
                <div className="flex items-center gap-3 mb-2">
                  {item.failed === 0 ? (
                    <CheckCircle className="w-5 h-5 text-green-500" />
                  ) : (
                    <XCircle className="w-5 h-5 text-yellow-500" />
                  )}
                  <h4 className="font-semibold">{item.type}</h4>
                </div>
                <div className="text-sm text-gray-600 space-y-1">
                  <div className="flex items-center gap-2">
                    <Clock className="w-4 h-4" />
                    {item.timestamp}
                  </div>
                  <div>
                    {item.vms} VMs: {item.success} successful, {item.failed} failed
                  </div>
                </div>
              </div>
              <button className="text-blue-600 hover:text-blue-700 text-sm font-medium">
                View Details
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
