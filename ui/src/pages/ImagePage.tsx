import React, { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { updateFavorite } from '../api/images';
import DeleteImageButton from '../components/DeleteImageButton';
import DuplicateImageNotice from '../components/DuplicateImageNotice';
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
            <span className={`${colorClass} font-medium leading-tight`}>{tag}</span>
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

  useEffect(() => {
    const fetchImage = async () => {
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
    };

    if (id) fetchImage();
  }, [id]);

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

  if (!imageData.main_data) {
    return (
      <DuplicateImageNotice
        fileName={imageData.file_name}
        hash={imageData.hash}
        imageId={imageData.id}
        originalImageId={imageData.has_duplicate ?? undefined}
      />
    );
  }

  const post = imageData.main_data;

  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans">
      <TopBar />

      <div className="flex flex-1 overflow-hidden">
        <aside className="w-72 bg-[#1c1c24] border-r border-[#2a2a35] overflow-y-auto p-4 flex-shrink-0 hide-scrollbar">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <FavoriteStar
                isFavorite={imageData.is_favorite ?? false}
                onToggle={handleToggleFavorite}
              />
              <span className="text-sm text-gray-400">
                {(imageData.is_favorite ?? false) ? 'Favorited' : 'Favorite'}
              </span>
            </div>
            <DeleteImageButton imageId={imageData.id} redirectTo="/" />
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
              {post.source && (
                <p className="truncate">
                  Source:{' '}
                  <a
                    href={post.source}
                    target="_blank"
                    rel="noreferrer"
                    className="text-[#60a5fa] hover:underline"
                  >
                    {post.source}
                  </a>
                </p>
              )}
              <p>
                Rating: <span className="text-gray-400 capitalize">{post.rating}</span>
              </p>
            </div>
          </div>
        </aside>

        <main className="flex-1 flex items-center justify-center p-8 overflow-hidden">
          <div className="relative max-w-full max-h-full flex items-center justify-center border-2 border-gray-600 rounded-[2rem] p-4">
            <img
              src={`/images/${imageData.file_name}`}
              alt={`Post ${post.id}`}
              className="max-w-full max-h-[85vh] object-contain rounded-xl"
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
