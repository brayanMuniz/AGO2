import React, { useEffect, useMemo, useState } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import SearchAutocomplete from '../components/SearchAutocomplete';
import {
  appendTagToQuery,
  toSearchQuery,
  type TagCategory,
  type TagCategoryKey,
  type TagSuggestion,
} from '../utils/searchTags';

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
  main_data: Post | null;
  thumbnail_path: string;
}

type TagCount = { name: string; count: number };

const CATEGORY_COLORS: Record<TagCategory, string> = {
  artist: 'text-[#fca5a5]',
  copyright: 'text-[#c084fc]',
  character: 'text-[#4ade80]',
  general: 'text-[#60a5fa]',
  meta: 'text-[#fb923c]',
};

const SIDEBAR_SECTIONS: { category: TagCategory; postKey: TagCategoryKey }[] = [
  { category: 'artist', postKey: 'tags_artist' },
  { category: 'copyright', postKey: 'tags_copyright' },
  { category: 'character', postKey: 'tags_character' },
  { category: 'general', postKey: 'tags_general' },
  { category: 'meta', postKey: 'tags_meta' },
];

function aggregateTags(images: ImageData[], category: TagCategoryKey): TagCount[] {
  const counts: Record<string, number> = {};
  images.forEach((img) => {
    if (!img.main_data) return;
    const tags = img.main_data[category] as string[];
    if (tags) {
      tags.forEach((tag) => {
        counts[tag] = (counts[tag] || 0) + 1;
      });
    }
  });

  return Object.entries(counts)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count);
}

function buildSuggestionsFromImages(images: ImageData[]): TagSuggestion[] {
  const suggestions: TagSuggestion[] = [];

  SIDEBAR_SECTIONS.forEach(({ category, postKey }) => {
    aggregateTags(images, postKey).forEach(({ name, count }) => {
      suggestions.push({ name, category, count });
    });
  });

  return suggestions;
}

function mergeSuggestions(...lists: TagSuggestion[][]): TagSuggestion[] {
  const merged = new Map<string, TagSuggestion>();

  lists.flat().forEach((tag) => {
    const key = `${tag.category}:${tag.name}`;
    const existing = merged.get(key);
    if (!existing || (tag.count ?? 0) > (existing.count ?? 0)) {
      merged.set(key, tag);
    }
  });

  return Array.from(merged.values());
}

// --- Main Component ---
const SearchPage: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const tagsQuery = searchParams.get('tags') || '';

  const [searchInput, setSearchInput] = useState(tagsQuery);
  const [images, setImages] = useState<ImageData[]>([]);
  const [knownTags, setKnownTags] = useState<TagSuggestion[]>([]);
  const [apiSuggestions, setApiSuggestions] = useState<TagSuggestion[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setSearchInput(tagsQuery);
  }, [tagsQuery]);

  useEffect(() => {
    const fetchImages = async () => {
      if (!tagsQuery) {
        setImages([]);
        return;
      }

      setLoading(true);
      setError(null);

      try {
        const response = await fetch(`/api/search?tags=${encodeURIComponent(tagsQuery)}`);

        if (!response.ok) {
          throw new Error('Failed to search images.');
        }

        const data: ImageData[] = await response.json();
        setImages(data || []);

        const resultTags = buildSuggestionsFromImages(data || []);
        setKnownTags((prev) => mergeSuggestions(prev, resultTags));
      } catch (err: any) {
        setError(err.message || 'An unknown error occurred.');
      } finally {
        setLoading(false);
      }
    };

    fetchImages();
  }, [tagsQuery]);

  useEffect(() => {
    const token = searchInput.trim().split(/\s+/).pop() || '';
    if (!token) {
      setApiSuggestions([]);
      return;
    }

    const timeout = window.setTimeout(async () => {
      try {
        const response = await fetch(
          `/api/tags/autocomplete?query=${encodeURIComponent(token)}`,
        );
        if (!response.ok) return;

        const data: TagSuggestion[] = await response.json();
        setApiSuggestions(data || []);
      } catch {
        setApiSuggestions([]);
      }
    }, 250);

    return () => window.clearTimeout(timeout);
  }, [searchInput]);

  const suggestions = useMemo(
    () => mergeSuggestions(knownTags, apiSuggestions),
    [knownTags, apiSuggestions],
  );

  const runSearch = (displayQuery: string) => {
    const query = toSearchQuery(displayQuery);
    if (query) {
      setSearchParams({ tags: query });
    } else {
      setSearchParams({});
    }
  };

  const addTagToQuery = (category: TagCategory, tagName: string) => {
    const nextInput = appendTagToQuery(searchInput, category, tagName);
    setSearchInput(nextInput);
    runSearch(nextInput);
  };

  const renderTagList = (tags: TagCount[], category: TagCategory) => {
    if (!tags || tags.length === 0) return null;

    return tags.map((tag) => (
      <li key={`${category}:${tag.name}`}>
        <button
          type="button"
          onClick={() => addTagToQuery(category, tag.name)}
          className="flex w-full items-start text-[13px] hover:underline cursor-pointer text-left"
        >
          <span className={`${CATEGORY_COLORS[category]} font-medium leading-tight flex-1`}>
            {tag.name}
          </span>
          <span className="text-gray-500 ml-1">{tag.count}</span>
        </button>
      </li>
    ));
  };

  const sidebarTags = SIDEBAR_SECTIONS.map(({ category, postKey }) => ({
    category,
    tags: aggregateTags(images, postKey),
  }));

  return (
    <div className="min-h-screen bg-[#0e0e12] flex text-gray-300 font-sans">

      {/* LEFT SIDEBAR */}
      <aside className="w-72 bg-[#1c1c24] border-r border-[#2a2a35] h-screen flex flex-col flex-shrink-0">

        {/* Search Input Area */}
        <div className="p-4 border-b border-[#2a2a35]">
          <h2 className="font-bold text-gray-200 mb-2 text-sm">Search</h2>
          <SearchAutocomplete
            value={searchInput}
            onChange={setSearchInput}
            onSearch={runSearch}
            suggestions={suggestions}
          />
        </div>

        {/* Aggregated Tags Area */}
        <div className="flex-1 overflow-y-auto p-4 hide-scrollbar">
          <h2 className="font-bold text-gray-200 mb-2 text-sm">Tags</h2>
          {images.length === 0 && !loading && (
            <p className="text-sm text-gray-500">No tags to display.</p>
          )}
          <ul className="space-y-0.5">
            {sidebarTags.map(({ category, tags }) => renderTagList(tags, category))}
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
              {tagsQuery ? 'No images found for these tags.' : 'Enter tags to search.'}
            </div>
          ) : (
            <div className="flex flex-wrap gap-4 content-start">
              {images.map((img) => (
                <Link
                  to={`/image/${img.id}`}
                  key={img.id}
                  className="block relative group"
                >
                  <div className="border border-transparent group-hover:border-[#60a5fa] transition-colors bg-[#111115] p-1">
                    <img
                      src={img.thumbnail_path ? `${img.thumbnail_path}` : `/images/${img.file_name}`}
                      alt={`Post ${img.id}`}
                      className="object-contain"
                      style={{
                        maxWidth: '250px',
                        maxHeight: '250px',
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
