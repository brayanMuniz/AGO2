import React, { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { updateFavorite } from '../api/images';
import DeleteImageButton from '../components/DeleteImageButton';
import DuplicateImageNotice from '../components/DuplicateImageNotice';
import MetadataMatcher from '../components/MetadataMatcher';
import FavoriteStar from '../components/FavoriteStar';
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
  const [imageData, setImageData] = useState<ImageData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showMatcher, setShowMatcher] = useState(false);

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

  const handleToggleFavorite = async () => {
    if (!imageData) return;

    const previous = imageData.is_favorite;
    const next = !previous;
    setImageData({ ...imageData, is_favorite: next });

    try {
      await updateFavorite(imageData.id, next);
    } catch {
      setImageData({ ...imageData, is_favorite: previous });
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-[#111115] text-white flex flex-col">
        <TopBar />
        <div className="flex-1 flex items-center justify-center">
          <span className="text-gray-400 text-lg">Loading...</span>
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
        />
      );
    }

    return (
      <MetadataMatcher
        imageId={imageData.id}
        fileName={imageData.file_name}
        onMatchSelected={fetchImage}
      />
    );
  }

  // --- Show Matcher for Higher Quality Overlay ---
  if (showMatcher) {
    return (
      <MetadataMatcher
        imageId={imageData.id}
        fileName={imageData.file_name}
        onClose={() => setShowMatcher(false)}
        onMatchSelected={() => {
          setShowMatcher(false);
          fetchImage();
        }}
      />
    );
  }

  const post = imageData.main_data;

  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans">
      <TopBar />

      <div className="flex flex-1 overflow-hidden">
        <aside className="w-72 bg-[#1c1c24] border-r border-[#2a2a35] overflow-y-auto p-4 flex-shrink-0 hide-scrollbar">
          {/* TOP CONTROLS: Favorite | Search Quality | Delete */}
          <div className="flex items-center justify-between mb-4 bg-[#15151a] p-2 rounded-lg border border-[#2a2a35]">
            <div className="flex items-center">
              <FavoriteStar
                isFavorite={imageData.is_favorite ?? false}
                onToggle={handleToggleFavorite}
              />
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
              />
            </div>
          </div>

          <TagCategory title="Artist" tags={post.tags_artist} colorClass="text-[#fca5a5]" />
          <TagCategory title="Copyright" tags={post.tags_copyright} colorClass="text-[#c084fc]" />
          <TagCategory title="Character" tags={post.tags_character} colorClass="text-[#4ade80]" />
          <TagCategory title="General" tags={post.tags_general} colorClass="text-[#60a5fa]" />
          <TagCategory title="Meta" tags={post.tags_meta} colorClass="text-[#fb923c]" />

          <div className="mt-6 text-[13px]">
            <h3 className="font-bold text-gray-200 mb-2 text-base">Information</h3>
            <div className="space-y-1">
              <p>
                ID: <span className="text-gray-400">{post.id}</span>
              </p>
              <p>
                Size:{' '}
                <span className="text-[#60a5fa]">
                  {formatBytes(post.file_size)} .{imageData.file_name.split('.').pop()} (
                  {post.image_width}x{post.image_height})
                </span>
              </p>

              {/* --- NEW: Link to Danbooru using post.id --- */}
              {post.id && (
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
              )}

              <p>
                Rating: <span className="text-gray-400 capitalize">{post.rating}</span>
              </p>
            </div>
          </div>
        </aside>

        <main className="flex-1 flex items-center justify-center p-4 overflow-hidden min-h-0">
          <div className="relative w-full h-full flex items-center justify-center border-2 border-gray-600 rounded-[2rem] p-2">
            <img
              src={`/images/${imageData.file_name}`}
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
        </main>
      </div>
    </div>
  );
};

export default ImagePage;
