import React, { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import DuplicateImageNotice from '../components/DuplicateImageNotice';

// --- Types ---
// Mapped directly from your Go backend structs
interface Post {
  id: number;
  rating: string;
  source: string;
  image_height: number;
  image_width: number;
  file_size: number;

  tags_artist: string[];
  tags_character: string[];
  tags_copyright: string[];
  tags_general: string[];
  tags_meta: string[];

  tag_count: number;
  tag_count_artist: number;
  tag_count_character: number;
  tag_count_copyright: number;
  tag_count_general: number;
  tag_count_meta: number;
}

interface ImageData {
  id: number;
  file_name: string;
  hash: string;
  main_data: Post | null;
  thumbnail_path: string;
}

// --- Helper Components ---
// Reusable tag list component to keep the sidebar clean
const TagCategory = ({ title, tags, colorClass }: { title: string, tags: string[], colorClass: string }) => {
  if (!tags || tags.length === 0) return null;

  return (
    <div className="mb-4">
      <h3 className="font-bold text-gray-200 mb-1">{title}</h3>
      <ul className="space-y-0.5">
        {tags.map((tag, idx) => (
          <li key={idx} className="flex items-start text-[13px] hover:underline cursor-pointer">
            <span className="text-gray-500 mr-2 select-none">?</span>
            <span className={`${colorClass} font-medium leading-tight`}>{tag}</span>
          </li>
        ))}
      </ul>
    </div>
  );
};

// --- Main Component ---
const ImagePage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [imageData, setImageData] = useState<ImageData | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchImage = async () => {
      try {
        const response = await fetch(`/api/image/${id}`);

        if (!response.ok) {
          if (response.status === 404) {
            throw new Error("Image not found.");
          }
          throw new Error("Failed to load image data.");
        }

        const data: ImageData = await response.json();
        setImageData(data);
      } catch (err: any) {
        setError(err.message || "An unknown error occurred.");
      } finally {
        setLoading(false);
      }
    };

    if (id) {
      fetchImage();
    }
  }, [id]);

  // Utility to format bytes into readable format
  const formatBytes = (bytes: number) => {
    if (!bytes || bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(0)) + ' ' + sizes[i];
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-[#111115] text-white flex items-center justify-center">
        <span className="text-gray-400 text-lg">Loading...</span>
      </div>
    );
  }

  if (error || !imageData) {
    return (
      <div className="min-h-screen bg-[#111115] text-white flex items-center justify-center">
        <span className="text-red-400 text-lg">{error || "Data unavailable"}</span>
      </div>
    );
  }

  if (!imageData.main_data) {
    return (
      <DuplicateImageNotice
        fileName={imageData.file_name}
        hash={imageData.hash}
        imageId={imageData.id}
      />
    );
  }

  const post = imageData.main_data;

  return (
    <div className="min-h-screen bg-[#0e0e12] flex text-gray-300 font-sans">

      {/* LEFT SIDEBAR - Tags & Information */}
      <aside className="w-72 bg-[#1c1c24] border-r border-[#2a2a35] h-screen overflow-y-auto p-4 flex-shrink-0 hide-scrollbar">

        {/* Tags Sections */}
        <TagCategory title="Artist" tags={post.tags_artist} colorClass="text-[#fca5a5]" />
        <TagCategory title="Copyright" tags={post.tags_copyright} colorClass="text-[#c084fc]" />
        <TagCategory title="Character" tags={post.tags_character} colorClass="text-[#4ade80]" />
        <TagCategory title="General" tags={post.tags_general} colorClass="text-[#60a5fa]" />
        <TagCategory title="Meta" tags={post.tags_meta} colorClass="text-[#fb923c]" />

        {/* Information Section */}
        <div className="mt-6 text-[13px]">
          <h3 className="font-bold text-gray-200 mb-2 text-base">Information</h3>
          <div className="space-y-1">
            <p>ID: <span className="text-gray-400">{post.id}</span></p>
            <p>
              Size: <a href="#" className="text-[#60a5fa] hover:underline">
                {formatBytes(post.file_size)} .{imageData.file_name.split('.').pop()} ({post.image_width}x{post.image_height})
              </a>
            </p>
            {post.source && (
              <p className="truncate">
                Source: <a href={post.source} target="_blank" rel="noreferrer" className="text-[#60a5fa] hover:underline">
                  {post.source}
                </a>
              </p>
            )}
            <p>Rating: <span className="text-gray-400 capitalize">{post.rating}</span></p>
          </div>
        </div>
      </aside>

      {/* MAIN CONTENT AREA - Image Viewer */}
      <main className="flex-1 flex items-center justify-center p-8 h-screen overflow-hidden">
        <div className="relative max-w-full max-h-full flex items-center justify-center border-2 border-gray-600 rounded-[2rem] p-4">

          {/* Note: I'm making an assumption on your image delivery URL structure. 
              Update the src to match where your images are actually served from. */}
          <img
            src={`/images/${imageData.file_name}`}
            alt={`Post ${post.id}`}
            className="max-w-full max-h-[85vh] object-contain rounded-xl"
            onError={(e) => {
              // Fallback placeholder if image fails to load
              e.currentTarget.style.display = 'none';
              e.currentTarget.parentElement?.classList.add('min-w-[600px]', 'min-h-[400px]', 'bg-black');
            }}
          />

        </div>
      </main>

    </div>
  );
};

export default ImagePage;
