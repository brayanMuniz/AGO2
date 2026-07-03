import React, { useState, useEffect, useMemo } from 'react';
import TopBar from './TopBar';

interface Post {
  id: number;
  rating: string;
  source: string;
  image_height: number;
  image_width: number;
  file_size: number;
  preview_file_url?: string;
}

interface MatchRecord {
  post_id: number;
  score: number;
  post: Post;
}

interface MetadataMatcherProps {
  imageId: number;
  fileName: string;
  onMatchSelected?: (postId: number) => void;
}

const MetadataMatcher: React.FC<MetadataMatcherProps> = ({
  imageId,
  fileName,
  onMatchSelected,
}) => {
  const [matches, setMatches] = useState<MatchRecord[]>([]);
  const [minScore, setMinScore] = useState<number>(70);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  // States for the inline confirmation and API call
  const [confirmMatchId, setConfirmMatchId] = useState<number | null>(null);
  const [updating, setUpdating] = useState<boolean>(false);

  const formatBytes = (bytes: number) => {
    if (!bytes) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / k ** i).toFixed(2))} ${sizes[i]}`;
  };

  const fetchMatches = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(`/api/image/${imageId}/matches`);
      if (!response.ok) throw new Error('Failed to fetch matches');
      const data: MatchRecord[] = await response.json();
      setMatches(data || []);
    } catch (err: any) {
      setError(err.message || 'An error occurred while fetching matches.');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchMatches();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [imageId]);

  const filteredAndSortedMatches = useMemo(() => {
    return matches
      .filter((match) => match.score >= minScore)
      .sort((a, b) => b.score - a.score);
  }, [matches, minScore]);

  // Handles the API call when confirmed
  const handleConfirmMatch = async () => {
    if (!confirmMatchId) return;

    // Grab the full post object to send to the backend
    const match = matches.find((m) => m.post_id === confirmMatchId);
    if (!match) return;

    setUpdating(true);
    try {
      const response = await fetch(`/api/image/${imageId}`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          main_data: match.post, // Send the payload!
        }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to update image metadata.');
      }

      setConfirmMatchId(null);
      if (onMatchSelected) {
        onMatchSelected(confirmMatchId);
      } else {
        window.location.reload();
      }
    } catch (err: any) {
      alert(err.message || 'An error occurred while linking metadata.');
    } finally {
      setUpdating(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans h-screen overflow-hidden">
      <TopBar />

      <div className="flex flex-1 p-6 gap-6 min-h-0">

        {/* LEFT PANEL */}
        <div className="flex-1 bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-4 flex items-center justify-center relative min-h-0 h-full">
          <img
            src={`/images/${fileName}`}
            alt="Original file"
            className="max-w-full max-h-full object-contain rounded-lg"
            onError={(e) => {
              e.currentTarget.style.display = 'none';
              e.currentTarget.parentElement?.classList.add('bg-black');
            }}
          />
          <div className="absolute top-4 left-4 bg-black/60 px-3 py-1 rounded text-sm text-gray-400">
            {fileName}
          </div>
        </div>

        {/* RIGHT PANEL */}
        <div className="w-[450px] flex flex-col shrink-0 bg-[#1c1c24] border border-[#2a2a35] rounded-xl overflow-hidden h-full">

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
              onClick={fetchMatches}
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
            {loading && matches.length === 0 ? (
              <div className="text-center text-gray-500 mt-10">Searching for matches...</div>
            ) : error ? (
              <div className="text-center text-red-400 mt-10">{error}</div>
            ) : filteredAndSortedMatches.length === 0 ? (
              <div className="text-center text-gray-500 mt-10">No matches found above {minScore}%.</div>
            ) : (
              filteredAndSortedMatches.map((match) => (
                <div
                  key={match.post_id}
                  className="flex gap-4 p-3 bg-[#111115] border border-[#2a2a35] hover:border-[#60a5fa] rounded-lg transition-colors group"
                >

                  {/* Thumbnail */}
                  <div className="w-24 h-24 shrink-0 bg-[#1c1c24] border border-[#2a2a35] rounded overflow-hidden flex items-center justify-center text-gray-600 text-xs">
                    {match.post.preview_file_url ? (
                      <img
                        src={`/api/proxy-image?url=${encodeURIComponent(match.post.preview_file_url)}`}
                        alt={`Thumbnail for ${match.post_id}`}
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <span>ID: {match.post_id}</span>
                    )}
                  </div>

                  <div className="flex flex-col justify-center text-[13px] text-gray-400 w-full overflow-hidden">

                    {/* Header & Inline Button */}
                    <div className="flex justify-between items-center mb-1 h-8">
                      <span className="font-bold text-lg text-white">
                        <span className={match.score > 90 ? 'text-green-400' : match.score > 70 ? 'text-[#60a5fa]' : 'text-yellow-400'}>
                          {match.score.toFixed(1)}%
                        </span> Match
                      </span>

                      {confirmMatchId === match.post_id ? (
                        <div className="flex items-center gap-2">
                          <span className="text-xs text-[#60a5fa]">Link?</span>
                          <button
                            type="button"
                            onClick={handleConfirmMatch}
                            disabled={updating}
                            className="px-2 py-0.5 text-xs rounded bg-[#60a5fa] hover:bg-[#3b82f6] text-white disabled:opacity-50 cursor-pointer transition-colors"
                          >
                            {updating ? '...' : 'Yes'}
                          </button>
                          <button
                            type="button"
                            onClick={() => setConfirmMatchId(null)}
                            disabled={updating}
                            className="px-2 py-0.5 text-xs rounded bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 cursor-pointer transition-colors"
                          >
                            No
                          </button>
                        </div>
                      ) : (
                        <button
                          type="button"
                          onClick={() => setConfirmMatchId(match.post_id)}
                          className="px-3 py-1.5 bg-[#2a2a35] hover:bg-[#60a5fa] hover:text-white text-gray-300 rounded-md text-xs font-semibold transition-colors border border-transparent hover:border-[#3b82f6] cursor-pointer"
                        >
                          Match Metadata
                        </button>
                      )}
                    </div>

                    <div className="grid grid-cols-[80px_1fr] gap-x-2 gap-y-1 mt-1 truncate">
                      <span className="text-gray-500">Source:</span>
                      <a
                        href={`https://danbooru.donmai.us/posts/${match.post_id}`}
                        target="_blank"
                        rel="noreferrer"
                        className="text-[#60a5fa] hover:underline truncate block"
                      >
                        https://danbooru.donmai.us/posts/{match.post_id}
                      </a>

                      <span className="text-gray-500">Dimensions:</span>
                      <span className="text-gray-300">{match.post.image_width} × {match.post.image_height}</span>

                      <span className="text-gray-500">Size:</span>
                      <span className="text-gray-300">{formatBytes(match.post.file_size)}</span>
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

export default MetadataMatcher;
