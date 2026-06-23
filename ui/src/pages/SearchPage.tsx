import React, { useEffect, useState, FormEvent } from 'react';
import { useSearchParams, Link } from 'react-router-dom';

// --- Types ---
interface Post {
  id: number;
  tags_artist: string[];
  tags_character: string[];
  tags_copyright: string[];
  tags_general: string[];
  tags_meta: string[];
}

interface ImageData {
  id: number;
  file_name: string;
  hash: string;
  main_data: Post;
  thumbnail_path: string;
}

// Helper type for the sidebar tags
type TagCount = { name: string; count: number };

// --- Main Component ---
const SearchPage: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const tagsQuery = searchParams.get('tags') || '';

  const [searchInput, setSearchInput] = useState(tagsQuery);
  const [images, setImages] = useState<ImageData[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  // --- Fetch Logic ---
  useEffect(() => {
    const fetchImages = async () => {
      if (!tagsQuery) {
        setImages([]);
        return;
      }

      setLoading(true);
      setError(null);

      try {
        // We use encodeURIComponent to safely handle special characters like '+' or spaces
        const response = await fetch(`/api/search?tags=${encodeURIComponent(tagsQuery)}`);

        if (!response.ok) {
          throw new Error("Failed to search images.");
        }

        const data: ImageData[] = await response.json();
        setImages(data || []);
      } catch (err: any) {
        setError(err.message || "An unknown error occurred.");
      } finally {
        setLoading(false);
      }
    };

    fetchImages();
  }, [tagsQuery]);

  // --- Search Handler ---
  const handleSearchSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (searchInput.trim()) {
      setSearchParams({ tags: searchInput.trim() });
    } else {
      setSearchParams({});
    }
  };

  // --- Dynamic Sidebar Tag Extraction ---
  // This aggregates tags from the fetched images so the sidebar updates based on results
  const aggregateTags = (images: ImageData[], category: keyof Post): TagCount[] => {
    const counts: Record<string, number> = {};
    images.forEach(img => {
      const tags = img.main_data[category] as string[];
      if (tags) {
        tags.forEach(tag => {
          counts[tag] = (counts[tag] || 0) + 1;
        });
      }
    });

    // Convert to array and sort by count (highest first)
    return Object.entries(counts)
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count);
  };

  const artistTags = aggregateTags(images, 'tags_artist');
  const copyrightTags = aggregateTags(images, 'tags_copyright');
  const characterTags = aggregateTags(images, 'tags_character');
  const generalTags = aggregateTags(images, 'tags_general');
  const metaTags = aggregateTags(images, 'tags_meta');

  // --- Render Helpers ---
  const renderTagList = (tags: TagCount[], colorClass: string) => {
    if (!tags || tags.length === 0) return null;
    return tags.map((tag, idx) => (
      <li key={idx} className="flex items-start text-[13px] hover:underline cursor-pointer">
        <span className="text-gray-500 mr-2 select-none">?</span>
        <span className={`${colorClass} font-medium leading-tight flex-1`}>
          {tag.name}
        </span>
        <span className="text-gray-500 ml-1">{tag.count}</span>
      </li>
    ));
  };

  return (
    <div className="min-h-screen bg-[#0e0e12] flex text-gray-300 font-sans">

      {/* LEFT SIDEBAR */}
      <aside className="w-72 bg-[#1c1c24] border-r border-[#2a2a35] h-screen flex flex-col flex-shrink-0">

        {/* Search Input Area */}
        <div className="p-4 border-b border-[#2a2a35]">
          <h2 className="font-bold text-gray-200 mb-2 text-sm">Search</h2>
          <form onSubmit={handleSearchSubmit} className="flex">
            <input
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              className="flex-1 bg-[#2a2a35] text-white px-2 py-1 text-sm border border-transparent focus:border-blue-500 focus:outline-none rounded-l-sm"
              placeholder="Search tags..."
            />
            <button
              type="submit"
              className="bg-[#3a3a45] px-3 py-1 flex items-center justify-center text-gray-300 hover:text-white rounded-r-sm"
            >
              <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
              </svg>
            </button>
          </form>
        </div>

        {/* Aggregated Tags Area */}
        <div className="flex-1 overflow-y-auto p-4 hide-scrollbar">
          <h2 className="font-bold text-gray-200 mb-2 text-sm">Tags</h2>
          {images.length === 0 && !loading && (
            <p className="text-sm text-gray-500">No tags to display.</p>
          )}
          <ul className="space-y-0.5">
            {renderTagList(artistTags, "text-[#fca5a5]")}
            {renderTagList(copyrightTags, "text-[#c084fc]")}
            {renderTagList(characterTags, "text-[#4ade80]")}
            {renderTagList(generalTags, "text-[#60a5fa]")}
            {renderTagList(metaTags, "text-[#fb923c]")}
          </ul>
        </div>
      </aside>

      {/* MAIN CONTENT AREA */}
      <main className="flex-1 h-screen flex flex-col overflow-hidden">

        {/* Top Navbar area (Posts / Artist) */}
        <header className="h-10 border-b border-[#2a2a35] flex items-center px-4 shrink-0 gap-4 text-sm font-semibold">
          <span className="text-[#60a5fa] cursor-pointer">Posts</span>
          <span className="text-gray-400 hover:text-gray-200 cursor-pointer">Artist</span>
          <div className="ml-auto text-gray-400 text-xs">
            {images.length} result(s)
          </div>
        </header>

        {/* Gallery Grid */}
        <div className="flex-1 overflow-y-auto p-4 hide-scrollbar">
          {loading ? (
            <div className="flex justify-center items-center h-full text-gray-400">Searching...</div>
          ) : error ? (
            <div className="flex justify-center items-center h-full text-red-400">{error}</div>
          ) : images.length === 0 ? (
            <div className="flex justify-center items-center h-full text-gray-500">
              {tagsQuery ? "No images found for these tags." : "Enter tags to search."}
            </div>
          ) : (
            <div className="flex flex-wrap gap-4 content-start">
              {images.map((img) => (
                <Link
                  to={`/image/${img.main_data.id}`}
                  key={img.main_data.id}
                  className="block relative group"
                >
                  {/* Container representing the image card */}
                  <div className="border border-transparent group-hover:border-[#60a5fa] transition-colors bg-[#111115] p-1">
                    <img
                      src={img.thumbnail_path ? `${img.thumbnail_path}` : `/images/${img.file_name}`}
                      alt={`Post ${img.id}`}
                      className="object-contain"
                      style={{
                        maxWidth: '250px',
                        maxHeight: '250px'
                      }}
                      loading="lazy"
                    />
                  </div>
                </Link>
              ))}
            </div>
          )}
        </div>
      </main>

    </div>
  );
};

export default SearchPage;
