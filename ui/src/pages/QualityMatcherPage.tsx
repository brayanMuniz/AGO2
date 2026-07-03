import React, { useState, useEffect, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import TopBar from '../components/TopBar';

// Should these still be ?
interface Post {
  id: number;
  rating: string;
  source: string;
  image_height: number;
  image_width: number;
  file_size: number;
  file_url?: string;
  large_file_url?: string;
  preview_file_url?: string;
}

interface MatchRecord {
  post_id: number;
  score: number;
  post: Post;
}

interface BaseImageData {
  id: number;
  file_name: string;
  main_data?: Post;
}

const QualityMatcherPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [baseImage, setBaseImage] = useState<BaseImageData | null>(null);
  const [matches, setMatches] = useState<MatchRecord[]>([]);
  const [minScore, setMinScore] = useState<number>(85);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  const [confirmMatchId, setConfirmMatchId] = useState<number | null>(null);
  const [updating, setUpdating] = useState<boolean>(false);
  const [replaceFile, setReplaceFile] = useState<boolean>(true);

  const formatBytes = (bytes: number) => {
    if (!bytes) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / k ** i).toFixed(2))} ${sizes[i]}`;
  };

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      setError(null);
      try {
        // Fetch base image data first to get filename and current resolution
        const imgRes = await fetch(`/api/image/${id}`);
        if (!imgRes.ok) throw new Error('Failed to fetch image details');
        const imgData = await imgRes.json();
        setBaseImage(imgData);

        // Fetch matches
        const matchRes = await fetch(`/api/image/${id}/matches`);
        if (!matchRes.ok) throw new Error('Failed to fetch matches');
        const matchData = await matchRes.json();
        setMatches(matchData || []);
      } catch (err: any) {
        setError(err.message || 'An error occurred loading data.');
      } finally {
        setLoading(false);
      }
    };

    if (id) fetchData();
  }, [id]);

  const filteredAndSortedMatches = useMemo(() => {
    return matches
      .filter((match) => match.score >= minScore)
      .sort((a, b) => {
        const areaA = a.post.image_width * a.post.image_height;
        const areaB = b.post.image_width * b.post.image_height;
        if (areaB !== areaA) return areaB - areaA;
        return b.score - a.score;
      });
  }, [matches, minScore]);

  const handleConfirmMatch = async () => {
    if (!confirmMatchId) return;
    const match = matches.find((m) => m.post_id === confirmMatchId);
    if (!match) return;

    setUpdating(true);
    try {
      const response = await fetch(`/api/image/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          main_data: match.post,
          replace_image: replaceFile,
        }),
      });

      // Confirmed I am sending over the correct file_url
      console.log("Sending to backend:", match.post);

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to update image.');
      }

      setConfirmMatchId(null);
      navigate(`/image/${id}`); // Send them back to the image page on success
    } catch (err: any) {
      alert(err.message || 'An error occurred while updating the image.');
    } finally {
      setUpdating(false);
    }
  };

  const currentWidth = baseImage?.main_data?.image_width || 0;
  const currentHeight = baseImage?.main_data?.image_height || 0;

  const getResolutionColor = (matchWidth: number, matchHeight: number) => {
    const currentArea = currentWidth * currentHeight;
    const matchArea = matchWidth * matchHeight;
    if (currentArea === 0) return 'text-gray-300';
    if (matchArea > currentArea) return 'text-green-400 font-bold';
    if (matchArea < currentArea) return 'text-red-400';
    return 'text-gray-300';
  };

  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans h-screen overflow-hidden">
      <TopBar />

      <div className="flex flex-1 p-6 gap-6 min-h-0">

        {/* LEFT PANEL */}
        <div className="flex-1 bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-4 flex items-center justify-center relative min-h-0 h-full flex-col">
          <button
            onClick={() => navigate(`/image/${id}`)}
            className="absolute top-4 left-4 bg-[#2a2a35] hover:bg-[#3a3a45] px-3 py-1.5 rounded text-sm text-gray-300 transition-colors cursor-pointer flex items-center gap-2"
          >
            &larr; Back to Image
          </button>

          {baseImage ? (
            <>
              <img
                src={`/images/${baseImage.file_name}`}
                alt="Original file"
                className="max-w-full max-h-[85%] object-contain rounded-lg mt-8"
                onError={(e) => {
                  e.currentTarget.style.display = 'none';
                  e.currentTarget.parentElement?.classList.add('bg-black');
                }}
              />
              <div className="mt-4 flex flex-wrap gap-4 text-sm text-gray-400 bg-black/40 px-4 py-2 rounded-lg">
                <span>File: <span className="text-white">{baseImage.file_name}</span></span>
                {currentWidth > 0 && currentHeight > 0 && (
                  <span>Current Res: <span className="text-white font-mono">{currentWidth} × {currentHeight}</span></span>
                )}
              </div>
            </>
          ) : (
            <span className="text-gray-500">Loading original image...</span>
          )}
        </div>

        {/* RIGHT PANEL */}
        <div className="w-[450px] shrink-0 bg-[#1c1c24] border border-[#2a2a35] rounded-xl flex flex-col overflow-hidden h-full">
          <div className="p-4 border-b border-[#2a2a35] flex items-center justify-between bg-[#15151a] shrink-0">
            <div className="flex flex-col gap-1 w-2/3">
              <label className="text-sm font-semibold text-gray-200 flex justify-between">
                <span>Minimum Match:</span>
                <span className="text-[#60a5fa]">{minScore}%</span>
              </label>
              <input
                type="range"
                min="0"
                max="100"
                step="5"
                value={minScore}
                onChange={(e) => setMinScore(Number(e.target.value))}
                className="w-full accent-[#60a5fa] cursor-pointer"
              />
            </div>

            <button
              onClick={() => {
                // simple hack to re-trigger the data fetch
                setLoading(true);
                fetch(`/api/image/${id}/matches`)
                  .then(res => res.json())
                  .then(data => setMatches(data || []))
                  .finally(() => setLoading(false));
              }}
              disabled={loading}
              className="p-2 bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 rounded-lg transition-colors flex items-center gap-2 cursor-pointer"
              title="Refresh Matches"
            >
              {loading ? (
                <span className="animate-spin h-5 w-5 border-2 border-[#60a5fa] border-t-transparent rounded-full block"></span>
              ) : (
                <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
              )}
            </button>
          </div>

          <div className="flex-1 overflow-y-auto p-4 space-y-4 hide-scrollbar min-h-0">
            {loading ? (
              <div className="text-center text-gray-500 mt-10">Searching for qualities...</div>
            ) : error ? (
              <div className="text-center text-red-400 mt-10">{error}</div>
            ) : filteredAndSortedMatches.length === 0 ? (
              <div className="text-center text-gray-500 mt-10">No matches found above {minScore}%.</div>
            ) : (
              filteredAndSortedMatches.map((match) => (
                <div key={match.post_id} className="flex gap-4 p-3 bg-[#111115] border border-[#2a2a35] hover:border-[#60a5fa] rounded-lg transition-colors group">
                  <div className="w-24 h-24 shrink-0 bg-[#1c1c24] border border-[#2a2a35] rounded overflow-hidden flex items-center justify-center text-gray-600 text-xs">
                    {match.post.preview_file_url ? (
                      <img src={`/api/proxy-image?url=${encodeURIComponent(match.post.preview_file_url)}`} alt="Thumb" className="w-full h-full object-cover" />
                    ) : (
                      <span>ID: {match.post_id}</span>
                    )}
                  </div>

                  <div className="flex flex-col justify-center text-[13px] text-gray-400 w-full overflow-hidden">
                    <div className="flex justify-between items-center mb-1 h-8">
                      <span className="font-bold text-lg text-white">
                        <span className={match.score > 90 ? 'text-green-400' : match.score > 70 ? 'text-[#60a5fa]' : 'text-yellow-400'}>
                          {match.score.toFixed(1)}%
                        </span> Match
                      </span>

                      {confirmMatchId === match.post_id ? (
                        <div className="flex flex-col items-end gap-1">
                          <label className="flex items-center gap-1 cursor-pointer text-xs">
                            <input type="checkbox" checked={replaceFile} onChange={(e) => setReplaceFile(e.target.checked)} className="accent-[#60a5fa]" />
                            Replace
                          </label>
                          <div className="flex gap-1">
                            <button onClick={handleConfirmMatch} disabled={updating} className="px-2 py-0.5 text-xs rounded bg-[#60a5fa] hover:bg-[#3b82f6] text-white cursor-pointer">
                              {updating ? '...' : 'Yes'}
                            </button>
                            <button onClick={() => setConfirmMatchId(null)} disabled={updating} className="px-2 py-0.5 text-xs rounded bg-[#2a2a35] hover:bg-[#3a3a45] text-white cursor-pointer">
                              No
                            </button>
                          </div>
                        </div>
                      ) : (
                        <button onClick={() => setConfirmMatchId(match.post_id)} className="px-3 py-1.5 bg-[#2a2a35] hover:bg-[#60a5fa] hover:text-white text-gray-300 rounded-md text-xs font-semibold cursor-pointer border border-transparent hover:border-[#3b82f6] transition-colors">
                          Upgrade
                        </button>
                      )}
                    </div>

                    <div className="grid grid-cols-[80px_1fr] gap-x-2 gap-y-1 mt-1 truncate">
                      <span className="text-gray-500">Res:</span>
                      <span className={`${getResolutionColor(match.post.image_width, match.post.image_height)} font-mono`}>
                        {match.post.image_width} × {match.post.image_height}
                      </span>
                      <span className="text-gray-500">Size:</span>
                      <span className="text-gray-300">{formatBytes(match.post.file_size)}</span>
                      <span className="text-gray-500">Source:</span>
                      <a href={`https://danbooru.donmai.us/posts/${match.post_id}`} target="_blank" rel="noreferrer" className="text-[#60a5fa] hover:underline truncate block">
                        Danbooru ↗
                      </a>
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default QualityMatcherPage;
