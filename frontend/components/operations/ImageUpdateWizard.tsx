'use client';

import { useState } from 'react';
import { Loader2, CheckCircle, XCircle, AlertCircle } from 'lucide-react';

interface ImageUpdateWizardProps {
  selectedVMs: string[];
  wsManager: any;
  onComplete: () => void;
}

interface ActionProgress {
  action_id: string;
  status: string;
  progress: number;
  message: string;
  timestamp: string;
}

interface ActionResult {
  action_id: string;
  success: boolean;
  message: string;
  error?: string;
  details?: any;
  timestamp: string;
}

export default function ImageUpdateWizard({ selectedVMs, wsManager, onComplete }: ImageUpdateWizardProps) {
  const [step, setStep] = useState<'config' | 'executing' | 'complete'>('config');
  const [formData, setFormData] = useState({
    namespace: 'default',
    deploymentName: '',
    containerName: '',
    newImage: '',
    autoRollback: true,
  });
  const [progress, setProgress] = useState<Record<string, ActionProgress>>({});
  const [results, setResults] = useState<Record<string, ActionResult>>({});
  const [overallProgress, setOverallProgress] = useState(0);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setStep('executing');

    // Subscribe to action progress and results
    const handleProgress = (data: ActionProgress) => {
      setProgress(prev => ({
        ...prev,
        [data.action_id]: data,
      }));
      
      // Calculate overall progress
      const allProgress = Object.values({ ...progress, [data.action_id]: data });
      const avg = allProgress.reduce((sum, p) => sum + p.progress, 0) / allProgress.length;
      setOverallProgress(Math.round(avg));
    };

    const handleResult = (data: ActionResult) => {
      setResults(prev => ({
        ...prev,
        [data.action_id]: data,
      }));
    };

    // Listen for messages
    if (wsManager) {
      wsManager.on('ACTION_PROGRESS', handleProgress);
      wsManager.on('ACTION_RESULT', handleResult);
    }

    // Send action request for each VM
    selectedVMs.forEach((vmCode, index) => {
      const actionId = `update-${Date.now()}-${index}`;
      
      const actionRequest = {
        action_id: actionId,
        type: 'k8s_update_image',
        target: {
          layer: 'kubernetes',
          entity_type: 'deployment',
          entity_id: formData.deploymentName,
          namespace: formData.namespace,
        },
        parameters: {
          image: formData.newImage,
          container: formData.containerName,
        },
        options: {
          auto_rollback: formData.autoRollback,
        },
      };

      if (wsManager) {
        wsManager.send('BROWSER_ACTION', actionRequest);
      }
    });

    // Wait for all results
    setTimeout(() => {
      setStep('complete');
    }, 10000); // Timeout after 10 seconds for demo
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'success':
        return <CheckCircle className="w-5 h-5 text-green-500" />;
      case 'failed':
        return <XCircle className="w-5 h-5 text-red-500" />;
      case 'in_progress':
        return <Loader2 className="w-5 h-5 text-blue-500 animate-spin" />;
      default:
        return <AlertCircle className="w-5 h-5 text-gray-400" />;
    }
  };

  const successCount = Object.values(results).filter(r => r.success).length;
  const failedCount = Object.values(results).filter(r => !r.success).length;

  return (
    <div className="space-y-6">
      <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
        <h3 className="font-semibold text-blue-900 mb-2">🚀 Update Container Image</h3>
        <p className="text-sm text-blue-700">
          Update the container image for a deployment across {selectedVMs.length} VM{selectedVMs.length !== 1 ? 's' : ''}.
          This will trigger a rolling update with automatic rollback on failure.
        </p>
      </div>

      {step === 'config' && (
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Namespace
            </label>
            <input
              type="text"
              value={formData.namespace}
              onChange={(e) => setFormData({ ...formData, namespace: e.target.value })}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              placeholder="default"
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
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              placeholder="frontend"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Container Name (optional)
            </label>
            <input
              type="text"
              value={formData.containerName}
              onChange={(e) => setFormData({ ...formData, containerName: e.target.value })}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              placeholder="Leave empty to update all containers"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              New Image
            </label>
            <input
              type="text"
              value={formData.newImage}
              onChange={(e) => setFormData({ ...formData, newImage: e.target.value })}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent"
              placeholder="myregistry/frontend:v2.1.0"
              required
            />
            <p className="text-xs text-gray-500 mt-1">
              Format: registry/image:tag
            </p>
          </div>

          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="autoRollback"
              checked={formData.autoRollback}
              onChange={(e) => setFormData({ ...formData, autoRollback: e.target.checked })}
              className="w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-blue-500"
            />
            <label htmlFor="autoRollback" className="text-sm text-gray-700">
              Automatically rollback on failure
            </label>
          </div>

          <div className="flex gap-3 pt-4">
            <button
              type="submit"
              className="flex-1 bg-blue-600 text-white py-2 px-4 rounded-lg hover:bg-blue-700 transition-colors font-medium"
            >
              Update Image on {selectedVMs.length} VM{selectedVMs.length !== 1 ? 's' : ''}
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
      )}

      {step === 'executing' && (
        <div className="space-y-4">
          <div className="bg-gray-50 rounded-lg p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-sm font-medium text-gray-700">Overall Progress</span>
              <span className="text-sm font-semibold text-gray-900">{overallProgress}%</span>
            </div>
            <div className="w-full bg-gray-200 rounded-full h-2">
              <div
                className="bg-blue-600 h-2 rounded-full transition-all duration-300"
                style={{ width: `${overallProgress}%` }}
              />
            </div>
          </div>

          <div className="space-y-2">
            {selectedVMs.map((vmCode, index) => {
              const actionId = `update-${Date.now()}-${index}`;
              const prog = progress[actionId];
              const result = results[actionId];

              return (
                <div key={vmCode} className="border border-gray-200 rounded-lg p-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      {getStatusIcon(prog?.status || result?.success ? 'success' : 'pending')}
                      <div>
                        <div className="font-medium text-sm">VM: {vmCode}</div>
                        <div className="text-xs text-gray-500">
                          {prog?.message || result?.message || 'Waiting...'}
                        </div>
                      </div>
                    </div>
                    {prog && (
                      <div className="text-sm font-medium text-gray-600">
                        {prog.progress}%
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {step === 'complete' && (
        <div className="space-y-4">
          <div className={`rounded-lg p-4 ${failedCount > 0 ? 'bg-yellow-50 border border-yellow-200' : 'bg-green-50 border border-green-200'}`}>
            <div className="flex items-center gap-3">
              {failedCount > 0 ? (
                <AlertCircle className="w-6 h-6 text-yellow-600" />
              ) : (
                <CheckCircle className="w-6 h-6 text-green-600" />
              )}
              <div>
                <h3 className={`font-semibold ${failedCount > 0 ? 'text-yellow-900' : 'text-green-900'}`}>
                  {failedCount > 0 ? 'Partially Complete' : 'All Updates Successful!'}
                </h3>
                <p className={`text-sm ${failedCount > 0 ? 'text-yellow-700' : 'text-green-700'}`}>
                  {successCount} successful, {failedCount} failed out of {selectedVMs.length} VMs
                </p>
              </div>
            </div>
          </div>

          <div className="space-y-2">
            {Object.entries(results).map(([actionId, result]) => (
              <div
                key={actionId}
                className={`border rounded-lg p-3 ${result.success ? 'border-green-200 bg-green-50' : 'border-red-200 bg-red-50'}`}
              >
                <div className="flex items-start gap-3">
                  {getStatusIcon(result.success ? 'success' : 'failed')}
                  <div className="flex-1">
                    <div className="font-medium text-sm">{result.message}</div>
                    {result.error && (
                      <div className="text-xs text-red-600 mt-1">{result.error}</div>
                    )}
                    {result.details && (
                      <div className="text-xs text-gray-600 mt-1">
                        {result.details.old_image} → {result.details.new_image}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>

          <button
            onClick={onComplete}
            className="w-full bg-blue-600 text-white py-2 px-4 rounded-lg hover:bg-blue-700 transition-colors font-medium"
          >
            Done
          </button>
        </div>
      )}
    </div>
  );
}
