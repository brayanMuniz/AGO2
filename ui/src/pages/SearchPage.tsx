import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { updateFavorite } from '../api/images';
import DeleteImageButton from '../components/DeleteImageButton';
import ExportAlbumModal from '../components/ExportAlbumModal';
import FavoriteStar from '../components/FavoriteStar';
import SearchAutocomplete from '../components/SearchAutocomplete';
import SearchFiltersPanel from '../components/SearchFiltersPanel';
import SearchTagPills from '../components/SearchTagPills';
import TopBar from '../components/TopBar';
import type { ImageData } from '../types/image';
import {
  addTagPill,
  buildSearchQuery,
  createTagPill,
  parseSearchQuery,
  removeTagPill,
  type SearchFilters,
} from '../utils/searchQuery';
import {
  type TagCategory,
  type TagCategoryKey,
  type TagSuggestion,
} from '../utils/searchTags';

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
    tags?.forEach((tag) => {
      counts[tag] = (counts[tag] || 0) + 1;
    });
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

const SearchPage: React.FC = () => {
  const [searchParams, setSearchParams] = useSearchParams();
  const tagsQuery = searchParams.get('tags') || '';

  const [filters, setFilters] = useState<SearchFilters>(() => parseSearchQuery(tagsQuery));
  const [draftInput, setDraftInput] = useState('');
  const [images, setImages] = useState<ImageData[]>([]);
  const [knownTags, setKnownTags] = useState<TagSuggestion[]>([]);
  const [apiSuggestions, setApiSuggestions] = useState<TagSuggestion[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [selectionMode, setSelectionMode] = useState(false);
  const [showExportModal, setShowExportModal] = useState(false);

  useEffect(() => {
    setFilters(parseSearchQuery(tagsQuery));
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
        setKnownTags((prev) => mergeSuggestions(prev, buildSuggestionsFromImages(data || [])));
      } catch (err: any) {
        setError(err.message || 'An unknown error occurred.');
      } finally {
        setLoading(false);
      }
    };

    fetchImages();
  }, [tagsQuery]);

  useEffect(() => {
    if (!draftInput.trim()) {
      setApiSuggestions([]);
      return;
    }

    const timeout = window.setTimeout(async () => {
      try {
        const response = await fetch(
          `/api/tags/autocomplete?query=${encodeURIComponent(draftInput.trim())}`,
        );
        if (!response.ok) return;
        const data: TagSuggestion[] = await response.json();
        setApiSuggestions(data || []);
      } catch {
        setApiSuggestions([]);
      }
    }, 250);

    return () => window.clearTimeout(timeout);
  }, [draftInput]);

  const suggestions = useMemo(
    () => mergeSuggestions(knownTags, apiSuggestions),
    [knownTags, apiSuggestions],
  );

  const debounceRef = useRef<number | null>(null);

  const applyFilters = (nextFilters: SearchFilters, immediate = true) => {
    setFilters(nextFilters);

    const updateUrl = () => {
      const query = buildSearchQuery(nextFilters);
      if (query) {
        setSearchParams({ tags: query });
      } else {
        setSearchParams({});
      }
    };

    if (immediate) {
      if (debounceRef.current) window.clearTimeout(debounceRef.current);
      updateUrl();
      return;
    }

    if (debounceRef.current) window.clearTimeout(debounceRef.current);
    debounceRef.current = window.setTimeout(updateUrl, 350);
  };

  const handleAddTag = (category: TagCategory, name: string) => {
    applyFilters(addTagPill(filters, createTagPill(category, name)));
  };

  const handleRemoveTag = (pillId: string) => {
    applyFilters(removeTagPill(filters, pillId));
  };

  const handleToggleFavorite = async (imageId: number, currentValue: boolean) => {
    const nextValue = !currentValue;
    setImages((prev) =>
      prev.map((img) => (img.id === imageId ? { ...img, is_favorite: nextValue } : img)),
    );

    try {
      await updateFavorite(imageId, nextValue);
    } catch {
      setImages((prev) =>
        prev.map((img) => (img.id === imageId ? { ...img, is_favorite: currentValue } : img)),
      );
    }
  };

  const handleImageDeleted = (imageId: number) => {
    setImages((prev) => prev.filter((img) => img.id !== imageId));
    setSelectedIds((prev) => {
      const next = new Set(prev);
      next.delete(imageId);
      return next;
    });
  };

  const toggleSelected = (imageId: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(imageId)) next.delete(imageId);
      else next.add(imageId);
      return next;
    });
  };

  const renderTagList = (tags: TagCount[], category: TagCategory) => {
    if (!tags.length) return null;

    return tags.map((tag) => (
      <li key={`${category}:${tag.name}`}>
        <button
          type="button"
          onClick={() => handleAddTag(category, tag.name)}
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

  const selectedImageIds = Array.from(selectedIds);

  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans">
      <TopBar />

      <div className="flex flex-1 overflow-hidden">
        <aside className="w-72 bg-[#1c1c24] border-r border-[#2a2a35] flex flex-col flex-shrink-0">
          <div className="p-4 border-b border-[#2a2a35]">
            <h2 className="font-bold text-gray-200 mb-2 text-sm">Search</h2>
            <SearchAutocomplete
              draftInput={draftInput}
              onDraftChange={setDraftInput}
              onAddTag={handleAddTag}
              suggestions={suggestions}
            />
            <SearchTagPills tags={filters.tags} onRemove={handleRemoveTag} />
            <SearchFiltersPanel
              filters={filters}
              onChange={(next) => applyFilters(next, true)}
              onSliderChange={(next) => applyFilters(next, false)}
            />
          </div>

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

        <main className="flex-1 flex flex-col overflow-hidden">
          <div className="h-10 border-b border-[#2a2a35] flex items-center px-4 shrink-0 gap-3 text-xs">
            <span className="text-gray-400">{images.length} result(s)</span>
            <div className="ml-auto flex items-center gap-2">
              <button
                type="button"
                onClick={() => {
                  setSelectionMode((prev) => !prev);
                  setSelectedIds(new Set());
                }}
                className={`px-2.5 py-1 rounded border transition-colors ${
                  selectionMode
                    ? 'border-[#60a5fa] text-[#93c5fd] bg-[#60a5fa]/10'
                    : 'border-[#2a2a35] text-gray-400 hover:text-gray-200'
                }`}
              >
                Select
              </button>
              {selectionMode && selectedImageIds.length > 0 && (
                <button
                  type="button"
                  onClick={() => setShowExportModal(true)}
                  className="px-2.5 py-1 rounded border border-[#2a2a35] text-gray-300 hover:text-white hover:border-[#60a5fa]"
                >
                  Export ({selectedImageIds.length})
                </button>
              )}
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-4 hide-scrollbar">
            {loading ? (
              <div className="flex justify-center items-center h-full text-gray-400">Searching...</div>
            ) : error ? (
              <div className="flex justify-center items-center h-full text-red-400">{error}</div>
            ) : images.length === 0 ? (
              <div className="flex justify-center items-center h-full text-gray-500">
                {tagsQuery ? 'No images found for these tags.' : 'Add tags or filters to search.'}
              </div>
            ) : (
              <div className="flex flex-wrap gap-4 content-start">
                {images.map((img) => (
                  <div key={img.id} className="relative group">
                    {selectionMode && (
                      <label className="absolute top-2 left-2 z-10">
                        <input
                          type="checkbox"
                          checked={selectedIds.has(img.id)}
                          onChange={() => toggleSelected(img.id)}
                          className="accent-[#60a5fa]"
                        />
                      </label>
                    )}

                    <Link
                      to={selectionMode ? '#' : `/image/${img.id}`}
                      onClick={(event) => {
                        if (selectionMode) event.preventDefault();
                      }}
                      className="block relative"
                    >
                      <div className="border border-transparent group-hover:border-[#60a5fa] transition-colors bg-[#111115] p-1">
                        <img
                          src={img.thumbnail_path ? img.thumbnail_path : `/images/${img.file_name}`}
                          alt={`Post ${img.id}`}
                          className="object-contain"
                          style={{ maxWidth: '250px', maxHeight: '250px' }}
                          loading="lazy"
                        />
                      </div>

                      <div className="absolute top-2 right-2 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                        <FavoriteStar
                          isFavorite={img.is_favorite ?? false}
                          onToggle={() => handleToggleFavorite(img.id, img.is_favorite ?? false)}
                          size="sm"
                          className="bg-black/60"
                        />
                        <span className="bg-black/60 rounded-full">
                          <DeleteImageButton
                            imageId={img.id}
                            variant="icon"
                            onDeleted={() => handleImageDeleted(img.id)}
                            className="bg-transparent"
                          />
                        </span>
                      </div>
                    </Link>
                  </div>
                ))}
              </div>
            )}
          </div>
        </main>
      </div>

      {showExportModal && (
        <ExportAlbumModal
          imageIds={selectedImageIds}
          onClose={() => setShowExportModal(false)}
        />
      )}
    </div>
  );
};

export default SearchPage;
