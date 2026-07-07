import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { updateFavorite } from '../api/images';
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

// Helpers for the new Appearance Filters
const extractBrightness = (query: string): [number, number] | null => {
  const match = query.match(/(?:^|\s)brightness:(\d+)-(\d+)(?:\s|$)/);
  return match ? [parseInt(match[1], 10), parseInt(match[2], 10)] : null;
};

const extractColor = (query: string): string | null => {
  const match = query.match(/(?:^|\s)color:(#[0-9a-fA-F]{6})(?:\s|$)/);
  return match ? match[1] : null;
};

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

  // NEW: State to track deletion progress
  const [isDeleting, setIsDeleting] = useState(false);

  // Track which category tag lists are expanded
  const [expandedCategories, setExpandedCategories] = useState<Record<string, boolean>>({});

  // States for Sorting
  const [sortBy, setSortBy] = useState<'none' | 'created_at' | 'file_size' | 'dimensions'>('none');
  const [sortOrder, setSortOrder] = useState<'desc' | 'asc'>('desc');

  // States for Appearance UI
  const [brightness, setBrightness] = useState<[number, number]>([0, 255]);
  const [color, setColor] = useState<string>('#000000');
  const [hasColor, setHasColor] = useState(false);

  // Derived states for Status filters
  const isMissing = /(?:^|\s)is:missing(?:\s|$)/.test(tagsQuery);
  const isDuplicate = /(?:^|\s)is:duplicate(?:\s|$)/.test(tagsQuery);

  useEffect(() => {
    setFilters(parseSearchQuery(tagsQuery));

    // Sync Appearance UI with URL tags
    const b = extractBrightness(tagsQuery);
    setBrightness(b || [0, 255]);

    const c = extractColor(tagsQuery);
    if (c) {
      setColor(c);
      setHasColor(true);
    } else {
      setHasColor(false);
    }
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

  // Dedicated URL updater for Appearance tags (Color & Brightness) and Status flags
  const updateSpecialFilter = (prefix: string, newValue: string | null) => {
    let tokens = tagsQuery.split(/\s+/).filter(Boolean);
    tokens = tokens.filter((t) => !t.startsWith(prefix));

    if (newValue) tokens.push(newValue);

    const newQuery = tokens.join(' ');

    if (debounceRef.current) window.clearTimeout(debounceRef.current);
    debounceRef.current = window.setTimeout(() => {
      setSearchParams(newQuery ? { tags: newQuery } : {});
    }, 350);
  };

  const handleColorChange = (newColor: string) => {
    setColor(newColor);
    setHasColor(true);
    updateSpecialFilter('color:', `color:${newColor}`);
  };

  const handleClearColor = () => {
    setHasColor(false);
    updateSpecialFilter('color:', null);
  };

  const handleBrightnessChange = (min: number, max: number) => {
    setBrightness([min, max]);
    if (min === 0 && max === 255) {
      updateSpecialFilter('brightness:', null);
    } else {
      updateSpecialFilter('brightness:', `brightness:${min}-${max}`);
    }
  };

  const handleAddTag = (category: TagCategory, name: string) => {
    applyFilters(addTagPill(filters, createTagPill(category, name)));
  };

  const handleRemoveTag = (pillId: string) => {
    applyFilters(removeTagPill(filters, pillId));
  };

  const handleToggleFavorite = async (imageId: number, currentValue: boolean) => {
    const nextValue = !currentValue;
    const imageToUpdate = images.find((img) => img.id === imageId);

    const isFilteringFavorites =
      (filters as any).favorite === true ||
      /(?:^|\s)favorite:true/.test(tagsQuery) ||
      /(?:^|\s)favorite(?:\s|$)/.test(tagsQuery);

    const isFilteringNotFavorites =
      (filters as any).favorite === false ||
      /(?:^|\s)-favorite/.test(tagsQuery) ||
      /(?:^|\s)favorite:false/.test(tagsQuery);

    const shouldRemove =
      (nextValue === true && isFilteringNotFavorites) ||
      (nextValue === false && isFilteringFavorites);

    setImages((prev) => {
      if (shouldRemove) {
        return prev.filter((img) => img.id !== imageId);
      }
      return prev.map((img) => (img.id === imageId ? { ...img, is_favorite: nextValue } : img));
    });

    if (shouldRemove) {
      setSelectedIds((prev) => {
        const next = new Set(prev);
        next.delete(imageId);
        return next;
      });
    }

    try {
      await updateFavorite(imageId, nextValue);
    } catch {
      setImages((prev) => {
        if (shouldRemove && imageToUpdate) {
          return [...prev, imageToUpdate].sort((a, b) => b.id - a.id);
        }
        return prev.map((img) =>
          img.id === imageId ? { ...img, is_favorite: currentValue } : img
        );
      });
    }
  };

  const toggleSelected = (imageId: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(imageId)) next.delete(imageId);
      else next.add(imageId);
      return next;
    });
  };

  const toggleCategoryExpanded = (category: string) => {
    setExpandedCategories((prev) => ({
      ...prev,
      [category]: !prev[category],
    }));
  };

  // --- NEW: Batch Delete Handler ---
  const handleBatchDelete = async () => {
    const idsToDelete = Array.from(selectedIds);
    if (idsToDelete.length === 0) return;

    if (!window.confirm(`Are you sure you want to permanently delete ${idsToDelete.length} images? This cannot be undone.`)) {
      return;
    }

    setIsDeleting(true);
    try {
      const response = await fetch('/api/image/batch-delete', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids: idsToDelete }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.message || 'Failed to delete images.');
      }

      // Optimistically remove deleted items from UI
      setImages((prev) => prev.filter((img) => !selectedIds.has(img.id)));
      setSelectedIds(new Set());
      setSelectionMode(false);
    } catch (err: any) {
      alert(err.message || 'An error occurred during deletion.');
      // Optionally trigger a fresh fetch here to sync actual state
    } finally {
      setIsDeleting(false);
    }
  };

  // Sort Images
  const sortedImages = useMemo(() => {
    if (sortBy === 'none') {
      return images;
    }

    return [...images].sort((a, b) => {
      let valA = 0;
      let valB = 0;

      if (sortBy === 'file_size') {
        valA = a.main_data?.file_size || (a as any).file_size || 0;
        valB = b.main_data?.file_size || (b as any).file_size || 0;
      } else if (sortBy === 'dimensions') {
        const widthA = a.main_data?.image_width || (a as any).image_width || 0;
        const heightA = a.main_data?.image_height || (a as any).image_height || 0;
        valA = widthA * heightA;

        const widthB = b.main_data?.image_width || (b as any).image_width || 0;
        const heightB = b.main_data?.image_height || (b as any).image_height || 0;
        valB = widthB * heightB;
      } else if (sortBy === 'created_at') {
        valA = (a as any).created_at ? new Date((a as any).created_at).getTime() : a.id;
        valB = (b as any).created_at ? new Date((b as any).created_at).getTime() : b.id;
      }

      if (valA < valB) return sortOrder === 'asc' ? -1 : 1;
      if (valA > valB) return sortOrder === 'asc' ? 1 : -1;
      return 0;
    });
  }, [images, sortBy, sortOrder]);

  const renderTagList = (tags: TagCount[], category: TagCategory) => {
    if (!tags.length) return null;

    const isExpanded = expandedCategories[category];
    const visibleTags = isExpanded ? tags : tags.slice(0, 5);

    return (
      <React.Fragment key={category}>
        {visibleTags.map((tag) => (
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
        ))}
        {tags.length > 5 && (
          <li>
            <button
              type="button"
              onClick={() => toggleCategoryExpanded(category)}
              className="text-xs text-[#60a5fa] hover:text-[#93c5fd] transition-colors mt-0.5 mb-2 ml-1"
            >
              {isExpanded ? 'Show less' : `+ Show ${tags.length - 5} more`}
            </button>
          </li>
        )}
      </React.Fragment>
    );
  };

  const sidebarTags = SIDEBAR_SECTIONS.map(({ category, postKey }) => ({
    category,
    tags: aggregateTags(images, postKey),
  }));

  const selectedImageIds = Array.from(selectedIds);
  const allImagesSelected = images.length > 0 && selectedIds.size === images.length;

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

            {/* --- APPEARANCE FILTERS --- */}
            <div className="mt-4 pt-4 border-t border-[#2a2a35]">
              <h3 className="font-bold text-gray-200 mb-3 text-xs uppercase tracking-wider">Appearance</h3>

              {/* Color Palette */}
              <div className="mb-5">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm text-gray-400">Color Palette</span>
                  {hasColor && (
                    <button
                      onClick={handleClearColor}
                      className="text-xs text-red-400 hover:text-red-300 transition-colors"
                    >
                      Clear
                    </button>
                  )}
                </div>
                <div className="flex items-center gap-3">
                  <input
                    type="color"
                    value={hasColor ? color : '#000000'}
                    onChange={(e) => handleColorChange(e.target.value)}
                    className="w-8 h-8 rounded cursor-pointer bg-transparent border-0 p-0"
                    title="Pick a color"
                  />
                  <span className="text-xs text-gray-500 font-mono">
                    {hasColor ? color.toUpperCase() : 'None selected'}
                  </span>
                </div>
              </div>

              {/* Dual-Thumb Brightness Slider */}
              <div>
                <div className="flex items-center justify-between mb-3">
                  <span className="text-sm text-gray-400">Brightness</span>
                  {(brightness[0] > 0 || brightness[1] < 255) && (
                    <button
                      onClick={() => handleBrightnessChange(0, 255)}
                      className="text-xs text-red-400 hover:text-red-300 transition-colors"
                    >
                      Reset
                    </button>
                  )}
                </div>

                <div className="relative w-full h-1 mt-2 mb-4 flex items-center">
                  {/* Track Background */}
                  <div className="absolute w-full h-1 bg-[#2a2a35] rounded-full"></div>

                  {/* Active Track */}
                  <div
                    className="absolute h-1 bg-[#60a5fa] rounded-full pointer-events-none"
                    style={{
                      left: `${(brightness[0] / 255) * 100}%`,
                      right: `${100 - (brightness[1] / 255) * 100}%`
                    }}
                  ></div>

                  {/* Min Input */}
                  <input
                    type="range" min="0" max="255"
                    value={brightness[0]}
                    onChange={(e) => {
                      const val = Math.min(parseInt(e.target.value), brightness[1] - 1);
                      handleBrightnessChange(val, brightness[1]);
                    }}
                    className="absolute w-full appearance-none bg-transparent pointer-events-none z-20 focus:outline-none
                               [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:pointer-events-auto [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:bg-[#60a5fa] [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:cursor-pointer 
                               [&::-moz-range-thumb]:appearance-none [&::-moz-range-thumb]:pointer-events-auto [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:bg-[#60a5fa] [&::-moz-range-thumb]:border-none [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:cursor-pointer"
                  />

                  {/* Max Input */}
                  <input
                    type="range" min="0" max="255"
                    value={brightness[1]}
                    onChange={(e) => {
                      const val = Math.max(parseInt(e.target.value), brightness[0] + 1);
                      handleBrightnessChange(brightness[0], val);
                    }}
                    className="absolute w-full appearance-none bg-transparent pointer-events-none z-20 focus:outline-none
                               [&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:pointer-events-auto [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:bg-[#60a5fa] [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:cursor-pointer 
                               [&::-moz-range-thumb]:appearance-none [&::-moz-range-thumb]:pointer-events-auto [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:bg-[#60a5fa] [&::-moz-range-thumb]:border-none [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:cursor-pointer"
                  />
                </div>

                {/* Value display */}
                <div className="flex justify-between text-xs text-gray-500 font-mono">
                  <span>{brightness[0]}</span>
                  <span>{brightness[1]}</span>
                </div>
              </div>
            </div>

            {/* --- NEW STATUS FILTERS --- */}
            <div className="mt-4 pt-4 border-t border-[#2a2a35]">
              <h3 className="font-bold text-gray-200 mb-3 text-xs uppercase tracking-wider">Status</h3>
              <div className="flex gap-2">
                <button
                  onClick={() => updateSpecialFilter('is:missing', isMissing ? null : 'is:missing')}
                  className={`px-3 py-1.5 rounded text-xs transition-colors border ${isMissing
                    ? 'border-[#60a5fa] bg-[#60a5fa]/10 text-[#60a5fa]'
                    : 'border-[#2a2a35] text-gray-400 hover:text-gray-200 hover:border-gray-500'
                    }`}
                >
                  Missing Data
                </button>
                <button
                  onClick={() => updateSpecialFilter('is:duplicate', isDuplicate ? null : 'is:duplicate')}
                  className={`px-3 py-1.5 rounded text-xs transition-colors border ${isDuplicate
                    ? 'border-[#60a5fa] bg-[#60a5fa]/10 text-[#60a5fa]'
                    : 'border-[#2a2a35] text-gray-400 hover:text-gray-200 hover:border-gray-500'
                    }`}
                >
                  Duplicate
                </button>
              </div>
            </div>
            {/* --- END NEW STATUS FILTERS --- */}

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
          <div className="h-12 border-b border-[#2a2a35] flex items-center px-4 shrink-0 gap-4 text-xs">

            {/* SORTING CONTROLS */}
            <div className="flex items-center gap-2 pr-4 border-r border-[#2a2a35]">
              <span className="text-gray-500 font-medium">Sort by:</span>
              <select
                value={sortBy}
                onChange={(e) => setSortBy(e.target.value as 'none' | 'created_at' | 'file_size' | 'dimensions')}
                className="bg-[#1c1c24] border border-[#2a2a35] text-gray-300 rounded px-2 py-1.5 outline-none focus:border-[#60a5fa] transition-colors cursor-pointer"
              >
                <option value="none">None (Default)</option>
                <option value="created_at">Date Added</option>
                <option value="file_size">File Size</option>
                <option value="dimensions">Dimensions</option>
              </select>
              <button
                type="button"
                onClick={() => setSortOrder(prev => prev === 'asc' ? 'desc' : 'asc')}
                disabled={sortBy === 'none'}
                className={`w-7 h-7 flex items-center justify-center border rounded transition-colors ${sortBy === 'none'
                  ? 'bg-[#15151a] border-[#2a2a35] text-gray-600 cursor-not-allowed'
                  : 'bg-[#1c1c24] border-[#2a2a35] hover:border-[#60a5fa] hover:text-[#60a5fa] text-gray-300 cursor-pointer'
                  }`}
                title={sortBy === 'none' ? 'Sorting disabled' : sortOrder === 'asc' ? 'Ascending' : 'Descending'}
              >
                {sortOrder === 'asc' ? '↑' : '↓'}
              </button>
            </div>

            <span className="text-gray-400">{images.length} result(s)</span>

            <div className="ml-auto flex items-center gap-2">
              {selectionMode && (
                <button
                  type="button"
                  onClick={() => {
                    if (allImagesSelected) {
                      setSelectedIds(new Set());
                    } else {
                      setSelectedIds(new Set(images.map((img) => img.id)));
                    }
                  }}
                  disabled={isDeleting}
                  className="px-2.5 py-1.5 rounded border border-[#2a2a35] text-gray-400 hover:text-gray-200 transition-colors disabled:opacity-50"
                >
                  {allImagesSelected ? 'Deselect All' : 'Select All'}
                </button>
              )}

              <button
                type="button"
                onClick={() => {
                  setSelectionMode((prev) => !prev);
                  setSelectedIds(new Set());
                }}
                disabled={isDeleting}
                className={`px-2.5 py-1.5 rounded border transition-colors disabled:opacity-50 ${selectionMode
                  ? 'border-[#60a5fa] text-[#93c5fd] bg-[#60a5fa]/10'
                  : 'border-[#2a2a35] text-gray-400 hover:text-gray-200'
                  }`}
              >
                Select
              </button>

              {selectionMode && selectedImageIds.length > 0 && (
                <>
                  <button
                    type="button"
                    onClick={() => setShowExportModal(true)}
                    disabled={isDeleting}
                    className="px-2.5 py-1.5 rounded border border-[#2a2a35] text-gray-300 hover:text-white hover:border-[#60a5fa] disabled:opacity-50 transition-colors"
                  >
                    Export ({selectedImageIds.length})
                  </button>

                  {/* --- NEW: Batch Delete Button --- */}
                  <button
                    type="button"
                    onClick={handleBatchDelete}
                    disabled={isDeleting}
                    className="px-2.5 py-1.5 rounded border border-red-500/50 text-red-400 hover:bg-red-500/20 hover:text-red-300 hover:border-red-500 disabled:opacity-50 transition-colors"
                  >
                    {isDeleting ? 'Deleting...' : `Delete (${selectedImageIds.length})`}
                  </button>
                </>
              )}
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-4 hide-scrollbar">
            {loading ? (
              <div className="flex justify-center items-center h-full text-gray-400">Searching...</div>
            ) : error ? (
              <div className="flex justify-center items-center h-full text-red-400">{error}</div>
            ) : sortedImages.length === 0 ? (
              <div className="flex justify-center items-center h-full text-gray-500">
                {tagsQuery ? 'No images found for these tags.' : 'Add tags or filters to search.'}
              </div>
            ) : (
              <div className="flex flex-wrap gap-4 content-start">
                {sortedImages.map((img) => {
                  const isSelected = selectedIds.has(img.id);
                  return (
                    <div key={img.id} className="relative group">
                      <Link
                        to={selectionMode ? '#' : `/image/${img.id}`}
                        onClick={(event) => {
                          if (selectionMode) {
                            event.preventDefault();
                            // Prevent selection clicks while a batch delete is happening
                            if (!isDeleting) {
                              toggleSelected(img.id);
                            }
                          }
                        }}
                        className={`block relative rounded transition-all duration-200 ${selectionMode && isSelected
                          ? 'ring-2 ring-[#60a5fa] bg-[#60a5fa]/10 scale-[0.98]'
                          : 'hover:ring-1 hover:ring-[#60a5fa]'
                          } ${isDeleting ? 'pointer-events-none' : ''}`}
                      >
                        <div className={`bg-[#111115] p-1 rounded ${selectionMode && isSelected ? 'opacity-80' : ''}`}>
                          <img
                            src={img.thumbnail_path ? img.thumbnail_path : `/images/${img.file_name}`}
                            alt={`Post ${img.id}`}
                            className="object-contain"
                            style={{ maxWidth: '250px', maxHeight: '250px' }}
                            loading="lazy"
                          />
                        </div>

                        {/* Read-only visual checkbox indicator for selection mode */}
                        {selectionMode && (
                          <div className="absolute top-2 left-2 z-10 pointer-events-none bg-black/40 rounded-sm">
                            <input
                              type="checkbox"
                              readOnly
                              checked={isSelected}
                              className="accent-[#60a5fa] pointer-events-none m-1 block"
                            />
                          </div>
                        )}

                        {!selectionMode && (
                          <div
                            className="absolute top-2 right-2 flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity"
                            onClick={(e) => e.preventDefault()}
                          >
                            <FavoriteStar
                              isFavorite={img.is_favorite ?? false}
                              onToggle={() => handleToggleFavorite(img.id, img.is_favorite ?? false)}
                              size="sm"
                              className="bg-black/60"
                            />
                          </div>
                        )}
                      </Link>
                    </div>
                  );
                })}
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
