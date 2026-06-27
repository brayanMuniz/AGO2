import React, { useState } from 'react';

const SyncPage: React.FC = () => {
  const [isProcessing, setIsProcessing] = useState(false);

  const handleRunSync = async () => {
    setIsProcessing(true);
    try {
      await new Promise(resolve => setTimeout(resolve, 1500)); // Simulating API
    } catch (error) {
      console.error("Failed to sync:", error);
    } finally {
      setIsProcessing(false);
    }
  };

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-200">Sync Gallery</h1>
        <button
          onClick={handleRunSync}
          disabled={isProcessing}
          className="px-4 py-2 bg-[#2a2a35] border border-[#3a3a45] hover:border-[#60a5fa] text-white rounded-lg transition-colors disabled:opacity-50 cursor-pointer"
        >
          {isProcessing ? 'Processing...' : 'Run Sync'}
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6 flex-1">
        <div className="bg-[#0e0e12] border border-[#2a2a35] rounded-lg p-6 flex flex-col">
          <h2 className="font-bold text-gray-200 mb-4 border-b border-[#2a2a35] pb-2">Processed</h2>
          <div className="flex-1 flex items-center justify-center text-gray-500 text-sm">
            Data of what was successfully processed.
          </div>
        </div>

        <div className="bg-[#0e0e12] border border-[#2a2a35] rounded-lg p-6 flex flex-col">
          <h2 className="font-bold text-gray-200 mb-4 border-b border-[#2a2a35] pb-2">Needs Processing</h2>
          <div className="flex-1 flex items-center justify-center text-gray-500 text-sm">
            Data of what still needs processing.
          </div>
        </div>
      </div>
    </div>
  );
};

export default SyncPage;
