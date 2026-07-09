import React, { useState, useEffect, useMemo } from 'react';
import TopBar from './TopBar';
import DeleteImageButton from './DeleteImageButton';
import CustomMetadataModal from './CustomMetadataModal';

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
  tags_artist?: string[];
  tags_character?: string[];
  tags_copyright?: string[];
  tags_general?: string[];
  tags_meta?: string[];
}

interface MatchRecord {
  post_id: number;
  score: number;
  post: Post;
}

interface MetadataMatcherProps {
  imageId: number;
  fileName: string;
  fileSize?: number;
  onMatchSelected?: (postId: number) => void;
  onClose?: () => void;
  inQueue?: boolean;
  isFirst?: boolean;
  isLast?: boolean;
  queuePosition?: number;
  queueTotal?: number;
  onPrev?: () => void;
  onNext?: () => void;
}

const MetadataMatcher: React.FC<MetadataMatcherProps> = ({
  imageId,
  fileName,
  fileSize,
  onMatchSelected,
  onClose,
  inQueue,
  isFirst,
  isLast,
  queuePosition,
  queueTotal,
  onPrev,
  onNext,
}) => {
  const [matches, setMatches] = useState<MatchRecord[]>([]);
  const [minScore, setMinScore] = useState<number>(70);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  // Added states to capture the natural resolution of the base image
  const [currentWidth, setCurrentWidth] = useState<number>(0);
  const [currentHeight, setCurrentHeight] = useState<number>(0);

  // States for the inline confirmation and API call
  const [confirmMatchId, setConfirmMatchId] = useState<number | null>(null);
  const [confirmAction, setConfirmAction] = useState<'match' | 'replace' | null>(null);
  const [updating, setUpdating] = useState<boolean>(false);
  const [showCustomModal, setShowCustomModal] = useState<boolean>(false);
  const [fetchedFileSize, setFetchedFileSize] = useState<number | undefined>(fileSize);

  useEffect(() => {
    if (fileSize !== undefined) {
      setFetchedFileSize(fileSize);
      return;
    }
    fetch(`/api/image/${imageId}`)
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data?.file_size) setFetchedFileSize(data.file_size);
      })
      .catch(() => {});
  }, [imageId, fileSize]);

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

  // Dynamic color for resolution comparison
  const getResolutionColor = (matchWidth: number, matchHeight: number) => {
    const currentArea = currentWidth * currentHeight;
    const matchArea = matchWidth * matchHeight;
    if (currentArea === 0) return 'text-gray-300';
    if (matchArea > currentArea) return 'text-green-400 font-bold';
    if (matchArea < currentArea) return 'text-red-400';
    return 'text-gray-300';
  };

  // Handles the API call when confirmed
  const handleConfirmMatch = async () => {
    if (!confirmMatchId || !confirmAction) return;

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
          main_data: match.post,
          replace_image: confirmAction === 'replace', // Map the action correctly
        }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to update image metadata.');
      }

      setConfirmMatchId(null);
      setConfirmAction(null);

      if (inQueue && onNext && !isLast) {
        onNext();
      } else if (onMatchSelected) {
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
        <div className="flex-1 bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-4 flex flex-col items-center justify-center relative min-h-0 h-full">
          {onClose && (
            <button
              onClick={onClose}
              className="absolute top-4 left-4 bg-[#2a2a35] hover:bg-[#3a3a45] px-3 py-1.5 rounded text-sm text-gray-300 transition-colors cursor-pointer flex items-center gap-2"
            >
              &larr; Back to Image
            </button>
          )}

          <div className="absolute top-4 right-4 bg-[#15151a] p-1 rounded-lg border border-[#2a2a35]">
            <DeleteImageButton
              imageId={imageId}
              redirectTo="/"
              variant="icon"
              onDeleted={() => {
                if (inQueue && onNext && !isLast) {
                  onNext();
                }
              }}
            />
          </div>

          <img
            src={`/images/${fileName}?v=${currentWidth}x${currentHeight}`}
            alt="Original file"
            className="max-w-full max-h-[85%] object-contain rounded-lg mt-4"
            onLoad={(e) => {
              // Automatically grab resolution from the image itself
              setCurrentWidth(e.currentTarget.naturalWidth);
              setCurrentHeight(e.currentTarget.naturalHeight);
            }}
            onError={(e) => {
              e.currentTarget.style.display = 'none';
              e.currentTarget.parentElement?.classList.add('bg-black');
            }}
          />

          <div className="mt-4 flex flex-wrap gap-4 text-sm text-gray-400 bg-black/40 px-4 py-2 rounded-lg">
            <span>File: <span className="text-white">{fileName}</span></span>
            {fetchedFileSize !== undefined && (
              <span>Size: <span className="text-white">{formatBytes(fetchedFileSize)}</span></span>
            )}
            {currentWidth > 0 && currentHeight > 0 && (
              <span>Current Res: <span className="text-white font-mono">{currentWidth} × {currentHeight}</span></span>
            )}
          </div>
        </div>

        {/* RIGHT PANEL */}
        <div className="w-[500px] flex flex-col shrink-0 bg-[#1c1c24] border border-[#2a2a35] rounded-xl overflow-hidden h-full">
          <div className="p-4 border-b border-[#2a2a35] flex items-center justify-between bg-[#15151a] shrink-0">
            <div className="flex flex-col gap-1 w-3/5">
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

            <div className="flex items-center gap-2">
              <button
                onClick={fetchMatches}
                disabled={loading}
                className="p-2.5 bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 rounded-lg transition-colors flex items-center justify-center cursor-pointer"
                title="Refresh Matches"
              >
                {loading ? (
                  <span className="animate-spin h-4 w-4 border-2 border-[#60a5fa] border-t-transparent rounded-full block"></span>
                ) : (
                  <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                )}
              </button>

              <button
                type="button"
                onClick={() => setShowCustomModal(true)}
                className="px-3 py-2 bg-purple-500/20 hover:bg-purple-500/30 text-purple-300 border border-purple-500/30 rounded-lg text-xs font-bold transition-all shadow cursor-pointer"
              >
                + Custom
              </button>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-4 space-y-4 hide-scrollbar min-h-0">
            {loading && matches.length === 0 ? (
              <div className="text-center text-gray-500 mt-10">Searching for matches...</div>
            ) : error ? (
              <div className="text-center text-red-400 mt-10">{error}</div>
            ) : filteredAndSortedMatches.length === 0 ? (
              <div className="text-center text-gray-500 mt-10">No matches found above {minScore}%.</div>
            ) : (
              filteredAndSortedMatches.map((match) => {
                const canReplace = Boolean(match.post.file_url || match.post.large_file_url);
                const currentArea = currentWidth * currentHeight;
                const matchArea = match.post.image_width * match.post.image_height;

                const isExactMatch =
                  currentWidth > 0 &&
                  currentHeight > 0 &&
                  match.post.image_width === currentWidth &&
                  match.post.image_height === currentHeight &&
                  (fileSize === undefined || match.post.file_size === fileSize);

                const isHigherRes = !isExactMatch && currentArea > 0 && matchArea > currentArea;
                const isLowerRes = !isExactMatch && currentArea > 0 && matchArea < currentArea;

                return (
                  <div
                    key={match.post_id}
                    className={`flex gap-4 p-3 bg-[#111115] border rounded-lg transition-all group ${
                      isExactMatch
                        ? 'border-[#60a5fa]/80 shadow-[0_0_15px_rgba(96,165,250,0.12)]'
                        : 'border-[#2a2a35] hover:border-[#60a5fa]'
                    }`}
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
                      {/* Header & Inline Buttons */}
                      <div className="flex justify-between items-center mb-1 h-8">
                        <span className="font-bold text-lg text-white">
                          <span className={match.score > 90 ? 'text-green-400' : match.score > 70 ? 'text-[#60a5fa]' : 'text-yellow-400'}>
                            {match.score.toFixed(1)}%
                          </span> Match
                        </span>

                        {confirmMatchId === match.post_id ? (
                          <div className="flex items-center gap-2">
                            <span className="text-xs text-[#60a5fa] font-semibold">
                              {confirmAction === 'replace' ? 'Replace file?' : 'Match data?'}
                            </span>
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
                              onClick={() => {
                                setConfirmMatchId(null);
                                setConfirmAction(null);
                              }}
                              disabled={updating}
                              className="px-2 py-0.5 text-xs rounded bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 cursor-pointer transition-colors"
                            >
                              No
                            </button>
                          </div>
                        ) : (
                          <div className="flex items-center gap-2">
                            <button
                              type="button"
                              onClick={() => {
                                setConfirmMatchId(match.post_id);
                                setConfirmAction('match');
                              }}
                              className="px-3 py-1.5 bg-[#2a2a35] hover:bg-[#60a5fa] hover:text-white text-gray-300 rounded-md text-xs font-semibold transition-colors border border-transparent hover:border-[#3b82f6] cursor-pointer"
                            >
                              Match
                            </button>
                            <button
                              type="button"
                              disabled={!canReplace}
                              title={
                                !canReplace
                                  ? 'Danbooru file unavailable (taken down / deleted)'
                                  : isExactMatch
                                  ? 'Exact dimensions and file size match'
                                  : isHigherRes
                                  ? 'Higher resolution available – click to replace!'
                                  : isLowerRes
                                  ? 'Lower resolution match'
                                  : 'Replace file'
                              }
                              onClick={() => {
                                if (!canReplace) return;
                                setConfirmMatchId(match.post_id);
                                setConfirmAction('replace');
                              }}
                              className={`px-3 py-1.5 rounded-md text-xs font-semibold transition-colors border ${
                                !canReplace
                                  ? 'opacity-35 cursor-not-allowed bg-[#1c1c24] text-gray-500 border-transparent'
                                  : isExactMatch
                                  ? 'bg-[#60a5fa]/15 text-[#60a5fa] border-[#60a5fa] hover:bg-[#60a5fa] hover:text-white cursor-pointer'
                                  : isHigherRes
                                  ? 'bg-green-500/15 text-green-400 border-green-500 hover:bg-green-500 hover:text-white cursor-pointer'
                                  : isLowerRes
                                  ? 'bg-red-500/15 text-red-400 border-red-500 hover:bg-red-500 hover:text-white cursor-pointer'
                                  : 'bg-[#2a2a35] hover:text-white text-gray-300 border-transparent hover:border-red-400 hover:bg-red-500/20 cursor-pointer'
                              }`}
                            >
                              Replace
                            </button>
                          </div>
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
                        Danbooru ↗
                      </a>

                      <span className="text-gray-500">Res:</span>
                      <span className={`${getResolutionColor(match.post.image_width, match.post.image_height)} font-mono`}>
                        {match.post.image_width} × {match.post.image_height}
                      </span>

                      <span className="text-gray-500">Size:</span>
                      <span className="text-gray-300">{formatBytes(match.post.file_size)}</span>
                    </div>

                    {/* Bottom tag pills for fast recognition */}
                    <div className="mt-2.5 pt-2 border-t border-[#2a2a35]/60 flex flex-wrap gap-1.5 items-center">
                      {((match.post.tags_character && match.post.tags_character.length > 0) ||
                        (match.post.tags_copyright && match.post.tags_copyright.length > 0) ||
                        (match.post.tags_artist && match.post.tags_artist.length > 0)) ? (
                        <>
                          {match.post.tags_character?.map((tag) => (
                            <span
                              key={`char-${tag}`}
                              className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium bg-[#4ade80]/15 text-[#4ade80] border border-[#4ade80]/30"
                              title="Character"
                            >
                              {tag.replace(/_/g, ' ')}
                            </span>
                          ))}
                          {match.post.tags_copyright?.map((tag) => (
                            <span
                              key={`copy-${tag}`}
                              className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium bg-[#c084fc]/15 text-[#c084fc] border border-[#c084fc]/30"
                              title="Series / Copyright"
                            >
                              {tag.replace(/_/g, ' ')}
                            </span>
                          ))}
                          {match.post.tags_artist?.map((tag) => (
                            <span
                              key={`art-${tag}`}
                              className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium bg-[#fca5a5]/15 text-[#fca5a5] border border-[#fca5a5]/30"
                              title="Artist"
                            >
                              {tag.replace(/_/g, ' ')}
                            </span>
                          ))}
                        </>
                      ) : (match.post.tags_general && match.post.tags_general.length > 0) ? (
                        <>
                          <span className="text-[11px] text-gray-500 italic mr-0.5">General:</span>
                          {match.post.tags_general.slice(0, 6).map((tag) => (
                            <span
                              key={`gen-${tag}`}
                              className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium bg-[#60a5fa]/15 text-[#60a5fa] border border-[#60a5fa]/30"
                              title="General Tag"
                            >
                              {tag.replace(/_/g, ' ')}
                            </span>
                          ))}
                        </>
                      ) : (
                        <span className="text-xs text-gray-500 italic">
                          No character/series tag data available
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              );
            })
            )}
          </div>

          {inQueue && (
            <div className="p-3 border-t border-[#2a2a35] bg-[#15151a] flex items-center justify-between shrink-0">
              <span className="text-xs font-semibold text-gray-400">
                Queue: <span className="text-[#60a5fa]">{queuePosition}</span> / {queueTotal}
              </span>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={onPrev}
                  disabled={isFirst || !onPrev}
                  className="px-3 py-1.5 rounded bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 text-xs font-medium disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer transition-colors"
                >
                  &larr; Previous
                </button>
                <button
                  type="button"
                  onClick={onNext}
                  disabled={isLast || !onNext}
                  className="px-3 py-1.5 rounded bg-[#60a5fa] hover:bg-[#3b82f6] text-white text-xs font-medium disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer transition-colors"
                >
                  Next &rarr;
                </button>
              </div>
            </div>
          )}
        </div>
      </div>

      {showCustomModal && (
        <CustomMetadataModal
          imageId={imageId}
          fileName={fileName}
          onClose={() => setShowCustomModal(false)}
          onSaved={() => {
            setShowCustomModal(false);
            if (inQueue && onNext && !isLast) {
              onNext();
            } else if (onMatchSelected) {
              onMatchSelected(0);
            } else {
              window.location.reload();
            }
          }}
        />
      )}
    </div>
  );
};

export default MetadataMatcher;
