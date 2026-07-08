import React, { useState, useEffect, useRef } from 'react';

interface SyncStats {
  processed: number;
  auto_match: number;
  skipped: number;
  total_files: number;
}

const SyncGalleryWidget: React.FC = () => {
  const [isProcessing, setIsProcessing] = useState(false);
  const [jobId, setJobId] = useState<string | null>(null);
  const [stats, setStats] = useState<SyncStats>({
    processed: 0,
    auto_match: 0,
    skipped: 0,
    total_files: 0,
  });
  const [error, setError] = useState<string | null>(null);

  // Poll the backend when a jobId exists and isProcessing is true
  useEffect(() => {
    let intervalId: number;

    const pollStatus = async () => {
      if (!jobId) return;
      try {
        const response = await fetch(`/api/process-gallery/status?job_id=${jobId}`);
        if (!response.ok) throw new Error('Failed to fetch sync status.');

        const data = await response.json();

        if (data.stats) {
          setStats({
            processed: data.stats.processed || 0,
            auto_match: data.stats.auto_match || 0,
            skipped: data.stats.skipped || 0,
            total_files: data.total_files || 0,
          });
        }

        if (data.status === 'completed' || data.status === 'failed') {
          setIsProcessing(false);
          setJobId(null); // Stop polling
        }
      } catch (err: any) {
        console.error(err);
        setError(err.message || 'An error occurred while syncing.');
        setIsProcessing(false);
        setJobId(null);
      }
    };

    if (isProcessing && jobId) {
      // Poll immediately, then every 2 seconds
      pollStatus();
      intervalId = window.setInterval(pollStatus, 2000);
    }

    return () => {
      if (intervalId) window.clearInterval(intervalId);
    };
  }, [isProcessing, jobId]);

  const handleStartSync = async () => {
    setIsProcessing(true);
    setError(null);
    setStats({ processed: 0, auto_match: 0, skipped: 0, total_files: 0 });

    try {
      const response = await fetch('/api/process-gallery', {
        method: 'POST',
      });

      if (!response.ok) {
        throw new Error('Failed to start sync process.');
      }

      const data = await response.json();
      if (data.job_id) {
        setJobId(data.job_id);
      } else {
        throw new Error('No job_id returned from server.');
      }
    } catch (err: any) {
      setError(err.message || 'Failed to start sync.');
      setIsProcessing(false);
    }
  };

  // Calculate progress percentage safely
  const currentProgress = stats.total_files > 0
    ? Math.min(100, Math.round(((stats.processed + stats.skipped) / stats.total_files) * 100))
    : 0;

  const hasData = stats.total_files > 0;

  return (
    <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-6 mb-8">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-bold text-gray-200">Sync Gallery</h2>
          <p className="text-sm text-gray-500 mt-1">Scan and process new images into the database.</p>
        </div>

        {/* Stats display from your wireframe */}
        {hasData && (
          <div className="text-sm text-gray-400 font-mono flex items-center gap-4">
            <span>
              Auto Processed: <span className="text-white font-bold">{stats.auto_match} / {stats.processed}</span>
            </span>
            <span className="text-gray-600">|</span>
            <span>
              processed + skipped = total: <span className="text-white font-bold">{stats.processed} + {stats.skipped} = {stats.total_files}</span>
            </span>
          </div>
        )}
      </div>

      {/* Progress Bar Container */}
      {(isProcessing || hasData) && (
        <div className="flex items-center gap-6 p-6 border-2 border-[#2a2a35] rounded-[1.5rem] bg-[#0e0e12] mb-6">
          {/* Inner Track */}
          <div className="flex-1 h-14 border border-[#2a2a35] rounded-xl p-1.5 relative overflow-hidden bg-black/50">
            {/* Fill */}
            <div
              className="h-full bg-[#2a2a35] rounded-lg transition-all duration-500 ease-out flex items-center justify-end px-2"
              style={{ width: `${currentProgress}%`, minWidth: currentProgress > 0 ? '1rem' : '0' }}
            >
              {/* Optional: Add a subtle pulse to the bar while processing */}
              {isProcessing && currentProgress < 100 && (
                <div className="absolute inset-0 bg-gradient-to-r from-transparent via-white/5 to-transparent animate-pulse" />
              )}
            </div>
          </div>

          {/* Percentage Text */}
          <div className="w-16 text-right font-mono text-xl text-gray-300">
            {currentProgress}%
          </div>
        </div>
      )}

      {error && <div className="text-red-400 text-sm mb-4">{error}</div>}

      <button
        onClick={handleStartSync}
        disabled={isProcessing}
        className="px-5 py-2 bg-[#2a2a35] border border-[#3a3a45] hover:border-[#60a5fa] hover:text-[#60a5fa] text-gray-300 rounded-lg transition-colors disabled:opacity-50 disabled:hover:border-[#3a3a45] disabled:hover:text-gray-300 cursor-pointer font-medium"
      >
        {isProcessing ? 'Syncing...' : hasData ? 'Run Sync Again' : 'Run Sync'}
      </button>
    </div>
  );
};

const GeneralSettings: React.FC = () => {
  return (
    <div className="h-full flex flex-col">
      <h1 className="text-2xl font-bold text-gray-200 mb-6">General Settings</h1>

      <div className="max-w-4xl">
        <SyncGalleryWidget />

        <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-6">
          <h2 className="text-lg font-bold text-gray-200 mb-2">Other Settings</h2>
          <p className="text-gray-500 text-sm">More application preferences will go here.</p>
        </div>
      </div>
    </div>
  );
};

export default GeneralSettings;
