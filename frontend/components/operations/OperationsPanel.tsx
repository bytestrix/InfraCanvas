'use client';

import { useState } from 'react';
import { X } from 'lucide-react';
import ImageUpdateWizard from './ImageUpdateWizard';
import DeploymentScaler from './DeploymentScaler';
import ActionHistory from './ActionHistory';

interface OperationsPanelProps {
  isOpen: boolean;
  onClose: () => void;
  selectedVMs: string[];
  wsManager: any;
}

type OperationType = 'update_image' | 'scale' | 'restart' | 'logs' | 'history' | null;

export default function OperationsPanel({ isOpen, onClose, selectedVMs, wsManager }: OperationsPanelProps) {
  const [selectedOperation, setSelectedOperation] = useState<OperationType>(null);

  if (!isOpen) return null;

  const operations = [
    {
      id: 'update_image',
      name: 'Update Image',
      description: 'Update container images across deployments',
      icon: '🚀',
      color: 'bg-blue-500',
    },
    {
      id: 'scale',
      name: 'Scale Deployment',
      description: 'Scale deployments up or down',
      icon: '📊',
      color: 'bg-green-500',
    },
    {
      id: 'restart',
      name: 'Restart',
      description: 'Restart deployments or pods',
      icon: '🔄',
      color: 'bg-yellow-500',
    },
    {
      id: 'logs',
      name: 'View Logs',
      description: 'View logs from pods',
      icon: '📝',
      color: 'bg-purple-500',
    },
    {
      id: 'history',
      name: 'History',
      description: 'View past operations',
      icon: '📜',
      color: 'bg-gray-500',
    },
  ];

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 z-50 flex items-center justify-center">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-4xl max-h-[90vh] overflow-hidden flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b">
          <div>
            <h2 className="text-xl font-semibold">DevOps Operations</h2>
            <p className="text-sm text-gray-600">
              {selectedVMs.length} VM{selectedVMs.length !== 1 ? 's' : ''} selected
            </p>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-100 rounded-lg transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6">
          {!selectedOperation ? (
            /* Operation Selection */
            <div className="grid grid-cols-2 gap-4">
              {operations.map((op) => (
                <button
                  key={op.id}
                  onClick={() => setSelectedOperation(op.id as OperationType)}
                  className="p-6 border-2 border-gray-200 rounded-lg hover:border-blue-500 hover:shadow-md transition-all text-left group"
                >
                  <div className="flex items-start gap-4">
                    <div className={`${op.color} w-12 h-12 rounded-lg flex items-center justify-center text-2xl group-hover:scale-110 transition-transform`}>
                      {op.icon}
                    </div>
                    <div className="flex-1">
                      <h3 className="font-semibold text-lg mb-1">{op.name}</h3>
                      <p className="text-sm text-gray-600">{op.description}</p>
                    </div>
                  </div>
                </button>
              ))}
            </div>
          ) : (
            /* Operation Forms */
            <div>
              <button
                onClick={() => setSelectedOperation(null)}
                className="mb-4 text-blue-600 hover:text-blue-700 flex items-center gap-2"
              >
                ← Back to operations
              </button>

              {selectedOperation === 'update_image' && (
                <ImageUpdateWizard
                  selectedVMs={selectedVMs}
                  wsManager={wsManager}
                  onComplete={() => setSelectedOperation(null)}
                />
              )}

              {selectedOperation === 'scale' && (
                <DeploymentScaler
                  selectedVMs={selectedVMs}
                  wsManager={wsManager}
                  onComplete={() => setSelectedOperation(null)}
                />
              )}

              {selectedOperation === 'history' && (
                <ActionHistory />
              )}

              {selectedOperation === 'restart' && (
                <div className="text-center py-12 text-gray-500">
                  Restart operation coming soon...
                </div>
              )}

              {selectedOperation === 'logs' && (
                <div className="text-center py-12 text-gray-500">
                  Log viewer coming soon...
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
