import React, { useEffect, useState, useCallback } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { updateFavorite } from '../api/images';
import DeleteImageButton from '../components/DeleteImageButton';
import DuplicateImageNotice from '../components/DuplicateImageNotice';
import MetadataMatcher from '../components/MetadataMatcher';
import FavoriteStar from '../components/FavoriteStar';
import CustomMetadataModal from '../components/CustomMetadataModal';
import TopBar from '../components/TopBar';
import type { ImageData } from '../types/image';

const TagCategory = ({
  title,
  tags,
  colorClass,
}: {
  title: string;
  tags: string[];
  colorClass: string;
}) => {
  if (!tags?.length) return null;

  return (
    <div className="mb-4">
      <h3 className="font-bold text-gray-200 mb-1">{title}</h3>
      <ul className="space-y-0.5">
        {tags.map((tag) => (
          <li key={tag} className="text-[13px] hover:underline cursor-pointer">
            <Link
              to={`/?tags=${encodeURIComponent(tag)}`}
              className={`${colorClass} font-medium leading-tight block`}
            >
              {tag}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
};

const ImagePage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [imageData, setImageData] = useState<ImageData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showMatcher, setShowMatcher] = useState(false);
  const [showCustomModal, setShowCustomModal] = useState(false);
  const [queueIds, setQueueIds] = useState<number[]>([]);

  useEffect(() => {
    try {
      const saved = sessionStorage.getItem('ago_queue');
      if (saved) {
        const parsed = JSON.parse(saved);
        if (Array.isArray(parsed.ids)) {
          setQueueIds(parsed.ids);
        }
      }
    } catch {
      // ignore
    }
  }, [id]);

  const currentIndex = queueIds.indexOf(Number(id));
  const inQueue = currentIndex !== -1 && queueIds.length > 0;
  const isFirst = currentIndex <= 0;
  const isLast = currentIndex >= queueIds.length - 1;
  const prevId = !isFirst ? queueIds[currentIndex - 1] : null;
  const nextId = !isLast ? queueIds[currentIndex + 1] : null;

  const goToPrev = useCallback(() => {
    if (prevId) {
      navigate(`/image/${prevId}?queue=true`);
    }
  }, [prevId, navigate]);

  const goToNext = useCallback(() => {
    if (nextId) {
      navigate(`/image/${nextId}?queue=true`);
    }
  }, [nextId, navigate]);

  const handleToggleFavorite = useCallback(async () => {
    if (!imageData) return;

    const previous = imageData.is_favorite;
    const next = !previous;
    setImageData({ ...imageData, is_favorite: next });

    try {
      await updateFavorite(imageData.id, next);
    } catch {
      setImageData({ ...imageData, is_favorite: previous });
    }
  }, [imageData]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const tag = (e.target as HTMLElement)?.tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      if (e.key === 'ArrowLeft' || e.key === 'h' || e.key === 'H') {
        goToPrev();
      } else if (e.key === 'ArrowRight' || e.key === 'l' || e.key === 'L') {
        goToNext();
      } else if (e.key === 'f' || e.key === 'F') {
        e.preventDefault();
        handleToggleFavorite();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [goToPrev, goToNext, handleToggleFavorite]);

  const fetchImage = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`/api/image/${id}`);
      if (!response.ok) {
        if (response.status === 404) throw new Error('Image not found.');
        throw new Error('Failed to load image data.');
      }

      const data: ImageData = await response.json();
      setImageData(data);
    } catch (err: any) {
      setError(err.message || 'An unknown error occurred.');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    if (id) fetchImage();
  }, [id, fetchImage]);

  const formatBytes = (bytes: number) => {
    if (!bytes) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / k ** i).toFixed(0))} ${sizes[i]}`;
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-[#111115] text-white flex flex-col">
        <TopBar />
        <div className="flex-1 flex items-center justify-center">
          <span className="text-gray-400">Loading image...</span>
        </div>
      </div>
    );
  }

  if (error || !imageData) {
    return (
      <div className="min-h-screen bg-[#111115] text-white flex flex-col">
        <TopBar />
        <div className="flex-1 flex items-center justify-center">
          <span className="text-red-400 text-lg">{error || 'Data unavailable'}</span>
        </div>
      </div>
    );
  }

  // --- Handling Missing Data and Duplicates ---
  if (!imageData.main_data) {
    if (imageData.has_duplicate) {
      return (
        <DuplicateImageNotice
          fileName={imageData.file_name}
          hash={imageData.hash}
          imageId={imageData.id}
          originalImageId={imageData.has_duplicate}
          inQueue={inQueue}
          isFirst={isFirst}
          isLast={isLast}
          onPrev={goToPrev}
          onNext={goToNext}
        />
      );
    }

    return (
      <MetadataMatcher
        imageId={imageData.id}
        fileName={imageData.file_name}
        fileSize={imageData.file_size}
        onMatchSelected={fetchImage}
        inQueue={inQueue}
        isFirst={isFirst}
        isLast={isLast}
        queuePosition={currentIndex + 1}
        queueTotal={queueIds.length}
        onPrev={goToPrev}
        onNext={goToNext}
      />
    );
  }

  // --- Show Matcher for Higher Quality Overlay ---
  if (showMatcher) {
    return (
      <MetadataMatcher
        imageId={imageData.id}
        fileName={imageData.file_name}
        fileSize={imageData.file_size}
        onClose={() => setShowMatcher(false)}
        onMatchSelected={() => {
          setShowMatcher(false);
          fetchImage();
        }}
        inQueue={inQueue}
        isFirst={isFirst}
        isLast={isLast}
        queuePosition={currentIndex + 1}
        queueTotal={queueIds.length}
        onPrev={goToPrev}
        onNext={goToNext}
      />
    );
  }

  const post = imageData.main_data;

  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans">
      <TopBar />

      <div className="flex flex-1 overflow-hidden">
        <aside className="w-72 bg-[#1c1c24] border-r border-[#2a2a35] overflow-y-auto p-4 flex-shrink-0 hide-scrollbar">
          {/* TOP CONTROLS: Favorite | Customize | Search Quality | Delete */}
          <div className="flex items-center justify-between mb-4 bg-[#15151a] p-2 rounded-lg border border-[#2a2a35]">
            <div className="flex items-center gap-1.5">
              <FavoriteStar
                isFavorite={imageData.is_favorite ?? false}
                onToggle={handleToggleFavorite}
              />
              <button
                type="button"
                onClick={() => setShowCustomModal(true)}
                className="p-1.5 text-gray-400 hover:text-purple-400 hover:bg-[#2a2a35] rounded-full transition-colors cursor-pointer"
                title="Customize Metadata"
              >
                <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                </svg>
              </button>
            </div>

            <div className="flex items-center gap-1">
              <button
                onClick={() => setShowMatcher(true)}
                className="rounded-full p-1.5 text-gray-400 hover:text-[#60a5fa] hover:bg-[#2a2a35] transition-colors cursor-pointer"
                title="Search for Higher Quality"
              >
                <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7" />
                </svg>
              </button>

              <DeleteImageButton
                imageId={imageData.id}
                redirectTo="/"
                variant="icon"
                onDeleted={() => {
                  if (inQueue && nextId) {
                    goToNext();
                    return true;
                  }
                  return false;
                }}
              />
            </div>
          </div>

          <div className="mb-6 pb-4 border-b border-[#2a2a35] text-[13px]">
            <h3 className="font-bold text-gray-200 mb-2 text-base">Information</h3>
            <div className="space-y-1">
              <p>
                ID: <span className="text-gray-400">{post.id || imageData.id}</span>
              </p>
              <p>
                Size:{' '}
                <span className="text-[#60a5fa]">
                  {formatBytes(post.file_size)} .{imageData.file_name.split('.').pop()} (
                  {post.image_width}x{post.image_height})
                </span>
              </p>

              {post.source === 'Custom' || post.id === 0 ? (
                <>
                  <p className="truncate">
                    Source:{' '}
                    <span className="text-purple-400 font-semibold">Custom</span>
                  </p>
                  {post.original_post_id && post.original_post_id !== '0' && (
                    <p className="truncate">
                      Danbooru:{' '}
                      <a
                        href={`https://danbooru.donmai.us/posts/${post.original_post_id}`}
                        target="_blank"
                        rel="noreferrer"
                        className="text-[#60a5fa] hover:underline"
                      >
                        #{post.original_post_id} ↗
                      </a>
                    </p>
                  )}
                </>
              ) : post.id ? (
                <p className="truncate">
                  Source:{' '}
                  <a
                    href={`https://danbooru.donmai.us/posts/${post.id}`}
                    target="_blank"
                    rel="noreferrer"
                    className="text-[#60a5fa] hover:underline"
                  >
                    Danbooru ↗
                  </a>
                </p>
              ) : post.source ? (
                <p className="truncate">
                  Source: <span className="text-gray-300">{post.source}</span>
                </p>
              ) : null}

              <p>
                Rating: <span className="text-gray-400 capitalize">{post.rating}</span>
              </p>
            </div>
          </div>

          <TagCategory title="Artist" tags={post.tags_artist} colorClass="text-[#fca5a5]" />
          <TagCategory title="Copyright" tags={post.tags_copyright} colorClass="text-[#c084fc]" />
          <TagCategory title="Character" tags={post.tags_character} colorClass="text-[#4ade80]" />
          <TagCategory title="General" tags={post.tags_general} colorClass="text-[#60a5fa]" />
          <TagCategory title="Meta" tags={post.tags_meta} colorClass="text-[#fb923c]" />
        </aside>

        <main className="flex-1 flex items-center justify-center p-4 overflow-hidden min-h-0 relative">
          <div className="relative w-full h-full flex items-center justify-center border-2 border-gray-600 rounded-[2rem] p-2">
            <img
              src={`/images/${imageData.file_name}?v=${imageData.file_size || ''}`}
              alt={`Post ${post.id}`}
              className="max-w-full max-h-full object-contain rounded-xl"
              onError={(event) => {
                event.currentTarget.style.display = 'none';
                event.currentTarget.parentElement?.classList.add(
                  'min-w-[600px]',
                  'min-h-[400px]',
                  'bg-black',
                );
              }}
            />
          </div>

          {inQueue && (
            <div className="absolute bottom-8 right-8 z-20 flex items-center gap-2 bg-[#15151a]/95 backdrop-blur border border-[#2a2a35] px-4 py-2.5 rounded-xl shadow-2xl">
              <span className="text-xs font-semibold text-gray-400 mr-2">
                Queue: <span className="text-[#60a5fa]">{currentIndex + 1}</span> / {queueIds.length}
              </span>
              <button
                type="button"
                onClick={goToPrev}
                disabled={isFirst}
                className="px-3 py-1.5 rounded-lg bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-200 text-xs font-medium disabled:opacity-30 disabled:cursor-not-allowed transition-colors cursor-pointer flex items-center gap-1"
              >
                &larr; Previous
              </button>
              <button
                type="button"
                onClick={goToNext}
                disabled={isLast}
                className="px-3 py-1.5 rounded-lg bg-[#60a5fa] hover:bg-[#3b82f6] text-white text-xs font-medium disabled:opacity-30 disabled:cursor-not-allowed transition-colors cursor-pointer flex items-center gap-1"
              >
                Next &rarr;
              </button>
            </div>
          )}
        </main>
      </div>

      {showCustomModal && (
        <CustomMetadataModal
          imageId={imageData.id}
          fileName={imageData.file_name}
          initialData={post}
          onClose={() => setShowCustomModal(false)}
          onSaved={() => {
            setShowCustomModal(false);
            fetchImage();
          }}
        />
      )}
    </div>
  );
};

export default ImagePage;
