'use client';

import { useState } from 'react';

interface DeploymentScalerProps {
  selectedVMs: string[];
  wsManager: any;
  onComplete: () => void;
}

export default function DeploymentScaler({ selectedVMs, wsManager, onComplete }: DeploymentScalerProps) {
  const [formData, setFormData] = useState({
    namespace: 'default',
    deploymentName: '',
    replicas: 3,
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    // Implementation similar to ImageUpdateWizard
    console.log('Scaling deployment:', formData);
  };

  return (
    <div className="space-y-6">
      <div className="bg-green-50 border border-green-200 rounded-lg p-4">
        <h3 className="font-semibold text-green-900 mb-2">📊 Scale Deployment</h3>
        <p className="text-sm text-green-700">
          Scale a deployment to a specific number of replicas across {selectedVMs.length} VM{selectedVMs.length !== 1 ? 's' : ''}.
        </p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Namespace
          </label>
          <input
            type="text"
            value={formData.namespace}
            onChange={(e) => setFormData({ ...formData, namespace: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Deployment Name
          </label>
          <input
            type="text"
            value={formData.deploymentName}
            onChange={(e) => setFormData({ ...formData, deploymentName: e.target.value })}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">
            Number of Replicas
          </label>
          <input
            type="number"
            min="0"
            value={formData.replicas}
            onChange={(e) => setFormData({ ...formData, replicas: parseInt(e.target.value) })}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-green-500 focus:border-transparent"
            required
          />
        </div>

        <div className="flex gap-3 pt-4">
          <button
            type="submit"
            className="flex-1 bg-green-600 text-white py-2 px-4 rounded-lg hover:bg-green-700 transition-colors font-medium"
          >
            Scale Deployment
          </button>
          <button
            type="button"
            onClick={onComplete}
            className="px-4 py-2 border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
