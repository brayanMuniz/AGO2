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

interface DuplicateFile {
  id: number;
  file_name: string;
  file_size: number;
  image_width: number;
  image_height: number;
  is_favorite: boolean;
  organized: boolean;
  has_duplicate?: number;
}

interface DuplicateGroup {
  hash: string;
  files: DuplicateFile[];
}

interface DuplicateScanResult {
  status: string;
  total_scanned: number;
  duplicate_groups: DuplicateGroup[];
  newly_marked: number;
  error?: string;
}

const FindDuplicatesWidget: React.FC = () => {
  const [isScanning, setIsScanning] = useState(false);
  const [result, setResult] = useState<DuplicateScanResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleScan = async () => {
    setIsScanning(true);
    setError(null);
    setResult(null);

    try {
      const response = await fetch('/api/find-duplicates', { method: 'POST' });
      if (!response.ok) {
        const data = await response.json().catch(() => null);
        throw new Error(data?.error || 'Failed to scan for duplicates.');
      }
      const data: DuplicateScanResult = await response.json();
      setResult(data);
    } catch (err: any) {
      setError(err.message || 'An error occurred while scanning.');
    } finally {
      setIsScanning(false);
    }
  };

  const formatFileSize = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const totalDuplicateFiles = result
    ? result.duplicate_groups.reduce((sum, g) => sum + g.files.length, 0)
    : 0;

  return (
    <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-6 mb-8">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-bold text-gray-200">Find Duplicates</h2>
          <p className="text-sm text-gray-500 mt-1">
            Scan your library for images with identical pixel content.
          </p>
        </div>

        {result && !isScanning && (
          <div className="text-sm text-gray-400 font-mono flex items-center gap-3">
            <span>
              Scanned: <span className="text-white font-bold">{result.total_scanned}</span>
            </span>
            <span className="text-gray-600">|</span>
            <span>
              Groups: <span className="text-white font-bold">{result.duplicate_groups.length}</span>
            </span>
            {result.newly_marked > 0 && (
              <>
                <span className="text-gray-600">|</span>
                <span>
                  Newly Marked: <span className="text-[#f59e0b] font-bold">{result.newly_marked}</span>
                </span>
              </>
            )}
          </div>
        )}
      </div>

      {/* Scanning spinner */}
      {isScanning && (
        <div className="flex items-center gap-3 p-4 border border-[#2a2a35] rounded-lg bg-[#0e0e12] mb-4">
          <div className="w-5 h-5 border-2 border-[#60a5fa] border-t-transparent rounded-full animate-spin" />
          <span className="text-sm text-gray-300">Scanning all files for duplicate pixel hashes…</span>
        </div>
      )}

      {error && <div className="text-red-400 text-sm mb-4 p-3 bg-red-500/10 border border-red-500/20 rounded-lg">{error}</div>}

      {/* Results */}
      {result && !isScanning && (
        <div className="mb-4">
          {result.duplicate_groups.length === 0 ? (
            <div className="p-4 border border-[#22c55e]/30 bg-[#22c55e]/5 rounded-lg text-sm text-[#22c55e] flex items-center gap-2">
              <span>✓</span> No duplicate images found. Your library is clean!
            </div>
          ) : (
            <>
              <div className="p-3 border border-[#f59e0b]/30 bg-[#f59e0b]/5 rounded-lg text-sm text-[#f59e0b] mb-4 flex items-center gap-2">
                <span>⚠</span>
                Found {result.duplicate_groups.length} duplicate group{result.duplicate_groups.length !== 1 ? 's' : ''} ({totalDuplicateFiles} total files).
                {result.newly_marked > 0 && (
                  <span className="ml-1 text-gray-300">
                    {result.newly_marked} file{result.newly_marked !== 1 ? 's' : ''} newly marked as duplicate.
                  </span>
                )}
              </div>

              <div className="max-h-[420px] overflow-y-auto custom-scrollbar space-y-3 pr-1">
                {result.duplicate_groups.map((group, gi) => (
                  <div key={group.hash} className="border border-[#2a2a35] rounded-lg bg-[#15151a] overflow-hidden">
                    <div className="px-4 py-2.5 bg-[#1a1a22] border-b border-[#2a2a35] flex items-center justify-between">
                      <span className="text-xs text-gray-400 font-mono">
                        Group {gi + 1} — Hash: <span className="text-gray-300">{group.hash.slice(0, 12)}…</span>
                      </span>
                      <span className="text-xs text-gray-500">{group.files.length} files</span>
                    </div>

                    <div className="divide-y divide-[#2a2a35]/50">
                      {group.files.map((file, fi) => {
                        const isOriginal = fi === 0 && file.has_duplicate == null;
                        return (
                          <div
                            key={file.id}
                            className={`px-4 py-2.5 flex items-center justify-between text-sm ${
                              isOriginal ? 'bg-[#22c55e]/5' : ''
                            }`}
                          >
                            <div className="flex items-center gap-3 min-w-0">
                              {isOriginal && (
                                <span className="text-[10px] font-bold uppercase tracking-wider text-[#22c55e] bg-[#22c55e]/10 px-1.5 py-0.5 rounded shrink-0">
                                  Original
                                </span>
                              )}
                              {file.has_duplicate != null && (
                                <span className="text-[10px] font-bold uppercase tracking-wider text-[#f59e0b] bg-[#f59e0b]/10 px-1.5 py-0.5 rounded shrink-0">
                                  Duplicate
                                </span>
                              )}
                              <a
                                href={`/image/${file.id}`}
                                className="text-[#60a5fa] hover:underline truncate"
                                title={file.file_name}
                              >
                                {file.file_name}
                              </a>
                            </div>
                            <div className="flex items-center gap-4 text-xs text-gray-500 shrink-0 ml-3">
                              <span>{file.image_width}×{file.image_height}</span>
                              <span>{formatFileSize(file.file_size)}</span>
                              <span className="font-mono text-gray-600">ID {file.id}</span>
                              {file.is_favorite && <span className="text-yellow-400" title="Favorited">★</span>}
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      )}

      <button
        onClick={handleScan}
        disabled={isScanning}
        className="px-5 py-2 bg-[#2a2a35] border border-[#3a3a45] hover:border-[#f59e0b] hover:text-[#f59e0b] text-gray-300 rounded-lg transition-colors disabled:opacity-50 disabled:hover:border-[#3a3a45] disabled:hover:text-gray-300 cursor-pointer font-medium"
      >
        {isScanning ? 'Scanning…' : result ? 'Scan Again' : 'Find Duplicates'}
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
        <FindDuplicatesWidget />

        <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-6">
          <h2 className="text-lg font-bold text-gray-200 mb-2">Other Settings</h2>
          <p className="text-gray-500 text-sm">More application preferences will go here.</p>
        </div>
      </div>
    </div>
  );
};

export default GeneralSettings;
