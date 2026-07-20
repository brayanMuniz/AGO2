import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams, Link } from 'react-router-dom';
import { updateFavorite } from '../api/images';
import ExportAlbumModal from '../components/ExportAlbumModal';
import FavoriteStar from '../components/FavoriteStar';
import SavedFiltersDropdown from '../components/SavedFiltersDropdown';
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

interface SavedPalette {
  id: number;
  name: string;
  colors: string[];
}

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

const extractBrightness = (query: string): [number, number] | null => {
  const match = query.match(/(?:^|\s)brightness:(\d+)-(\d+)(?:\s|$)/);
  return match ? [parseInt(match[1], 10), parseInt(match[2], 10)] : null;
};

const extractPaletteColors = (query: string): string[] => {
  const palMatch = query.match(/(?:^|\s)palette:([^\s]+)(?:\s|$)/);
  if (palMatch) {
    return palMatch[1].split(',').filter(Boolean);
  }
  const colMatch = query.match(/(?:^|\s)color:([^\s]+)(?:\s|$)/);
  if (colMatch) {
    return colMatch[1].split(',').filter(Boolean);
  }
  return [];
};

// Reusable Dual Slider Component styled to match your old UI perfectly
const sliderClasses = `absolute w-full appearance-none bg-transparent pointer-events-none z-20 focus:outline-none 
[&::-webkit-slider-thumb]:appearance-none [&::-webkit-slider-thumb]:pointer-events-auto [&::-webkit-slider-thumb]:w-4 [&::-webkit-slider-thumb]:h-4 [&::-webkit-slider-thumb]:bg-[#60a5fa] [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:cursor-pointer 
[&::-moz-range-thumb]:appearance-none [&::-moz-range-thumb]:pointer-events-auto [&::-moz-range-thumb]:w-4 [&::-moz-range-thumb]:h-4 [&::-moz-range-thumb]:bg-[#60a5fa] [&::-moz-range-thumb]:border-none [&::-moz-range-thumb]:rounded-full [&::-moz-range-thumb]:cursor-pointer`;

const RenderDualSlider = ({
  label, min, max, currentMin, currentMax, step, formatValue, onChange
}: {
  label: string; min: number; max: number; currentMin: number; currentMax: number; step: number;
  formatValue: (v: number) => string; onChange: (min: number, max: number) => void;
}) => {
  const isDefault = currentMin === min && currentMax === max;
  const valueText = isDefault ? 'Any' : `${formatValue(currentMin)} - ${formatValue(currentMax)}`;

  return (
    <div className="mb-4">
      <div className="flex items-center justify-between mb-2">
        <span className="text-xs font-semibold text-gray-400 uppercase tracking-wide">{label}</span>
        <div className="flex items-center gap-2">
          {!isDefault && (
            <button
              onClick={() => onChange(min, max)}
              className="text-xs text-red-400 hover:text-red-300 transition-colors cursor-pointer"
            >
              Reset
            </button>
          )}
          <span className="text-xs text-gray-500">{valueText}</span>
        </div>
      </div>
      <div className="relative w-full h-1 mt-1 flex items-center">
        <div className="absolute w-full h-1 bg-[#2a2a35] rounded-full"></div>
        <div
          className="absolute h-1 bg-[#60a5fa] rounded-full pointer-events-none"
          style={{
            left: `${((currentMin - min) / (max - min)) * 100}%`,
            right: `${100 - ((currentMax - min) / (max - min)) * 100}%`
          }}
        ></div>
        <input
          type="range" min={min} max={max} step={step} value={currentMin}
          onChange={(e) => onChange(Math.min(Number(e.target.value), currentMax - step), currentMax)}
          className={sliderClasses}
        />
        <input
          type="range" min={min} max={max} step={step} value={currentMax}
          onChange={(e) => onChange(currentMin, Math.max(Number(e.target.value), currentMin + step))}
          className={sliderClasses}
        />
      </div>
    </div>
  );
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
  const [isDeleting, setIsDeleting] = useState(false);
  const [isExtractingPalette, setIsExtractingPalette] = useState(false);

  const [expandedCategories, setExpandedCategories] = useState<Record<string, boolean>>({});

  const urlSort = (searchParams.get('sort') as any) || 'none';
  const urlOrder = (searchParams.get('order') as any) || 'desc';
  const urlLimit = parseInt(searchParams.get('limit') || '50', 10);
  const urlPage = parseInt(searchParams.get('page') || '1', 10);

  const [sortBy, setSortBy] = useState<string>(urlSort);
  const [sortOrder, setSortOrder] = useState<'desc' | 'asc'>(urlOrder);
  const [randomSeed, setRandomSeed] = useState(() => Math.random());

  const [pageSize, setPageSize] = useState<number>(urlLimit > 0 ? urlLimit : 50);
  const [currentPage, setCurrentPage] = useState<number>(urlPage > 0 ? urlPage : 1);
  const [totalResults, setTotalResults] = useState<number>(0);
  const [zoomSize, setZoomSize] = useState<number>(240);

  const [brightness, setBrightness] = useState<[number, number]>([0, 255]);
  const [color, setColor] = useState<string>('#000000');
  const [hasColor, setHasColor] = useState(false);
  const [vibePalette, setVibePalette] = useState<string[]>([]);
  const [savedPalettes, setSavedPalettes] = useState<SavedPalette[]>([]);
  const [isSavingPalette, setIsSavingPalette] = useState(false);
  const [newPaletteName, setNewPaletteName] = useState('');

  const [widthRange, setWidthRange] = useState<[number, number]>([0, 10500]);
  const [heightRange, setHeightRange] = useState<[number, number]>([0, 10500]);
  const [sizeRange, setSizeRange] = useState<[number, number]>([0, 21]);

  const isMissing = /(?:^|\s)is:missing(?:\s|$)/.test(tagsQuery);
  const isDuplicate = /(?:^|\s)is:duplicate(?:\s|$)/.test(tagsQuery);
  const isOrganized = /(?:^|\s)is:organized(?:\s|$)/.test(tagsQuery);
  const isUnorganized = /(?:^|\s)(?:is:unorganized|is:missing)(?:\s|$)/.test(tagsQuery);
  const isAnyStatus = !isOrganized && !isUnorganized;

  const updateUrlParams = (
    newTags?: string,
    newSort?: string,
    newOrder?: string,
    newLimit?: number,
    newPage?: number,
  ) => {
    const t = newTags !== undefined ? newTags : tagsQuery;
    const s = newSort !== undefined ? newSort : sortBy;
    const o = newOrder !== undefined ? newOrder : sortOrder;
    const l = newLimit !== undefined ? newLimit : pageSize;
    const p = newPage !== undefined ? newPage : currentPage;
    const params: Record<string, string> = {};
    if (t) params.tags = t;
    if (s && s !== 'none') {
      params.sort = s;
      if (o) params.order = o;
    }
    if (l && l !== 50) params.limit = l.toString();
    if (p && p > 1) params.page = p.toString();
    setSearchParams(params);
  };

  useEffect(() => {
    if (!searchParams.get('tags') && !searchParams.get('sort')) {
      const defaultFilterStr = localStorage.getItem('ago2_default_filter');
      if (defaultFilterStr) {
        try {
          const df = JSON.parse(defaultFilterStr);
          if (df && (df.query || (df.sortBy && df.sortBy !== 'none'))) {
            setSortBy(df.sortBy || 'none');
            setSortOrder(df.sortOrder || 'desc');
            updateUrlParams(df.query || '', df.sortBy || 'none', df.sortOrder || 'desc');
          }
        } catch (e) {
          console.error('Failed to load default filter:', e);
        }
      }
    }
  }, []);

  useEffect(() => {
    setFilters(parseSearchQuery(tagsQuery));

    const b = extractBrightness(tagsQuery);
    setBrightness(b || [0, 255]);

    const extractedColors = extractPaletteColors(tagsQuery);
    if (extractedColors.length > 0) {
      setVibePalette(extractedColors);
      setHasColor(true);
      setColor(extractedColors[0]);
    } else {
      setVibePalette([]);
      setHasColor(false);
    }

    const parseMinMax = (prefix: string, maxVal: number, divisor: number = 1): [number, number] => {
      let min = 0;
      let max = maxVal;
      const minMatch = tagsQuery.match(new RegExp(`(?:^|\\s)${prefix}:>=(\\d+)(?:\\s|$)`));
      if (minMatch) min = parseInt(minMatch[1], 10) / divisor;
      const maxMatch = tagsQuery.match(new RegExp(`(?:^|\\s)${prefix}:<=(\\d+)(?:\\s|$)`));
      if (maxMatch) max = parseInt(maxMatch[1], 10) / divisor;
      return [min, max];
    };

    setWidthRange(parseMinMax('width', 10500));
    setHeightRange(parseMinMax('height', 10500));
    setSizeRange(parseMinMax('size', 21, 1024 * 1024));
  }, [tagsQuery]);

  useEffect(() => {
    const fetchImages = async () => {
      if (!tagsQuery) {
        setImages([]);
        setTotalResults(0);
        return;
      }

      setLoading(true);
      setError(null);

      const offset = (currentPage - 1) * pageSize;
      const queryParams = new URLSearchParams({
        tags: tagsQuery,
        limit: pageSize.toString(),
        offset: offset.toString(),
      });
      if (sortBy && sortBy !== 'none') {
        queryParams.set('sort_by', sortBy);
        queryParams.set('sort_order', sortOrder);
      }

      try {
        const response = await fetch(`/api/search?${queryParams.toString()}`);
        if (!response.ok) {
          throw new Error('Failed to search images.');
        }

        const data = await response.json();
        const fetchedImages: ImageData[] = Array.isArray(data) ? data : (data.images || []);
        const count: number = Array.isArray(data) ? data.length : (data.total_count || 0);

        setImages(fetchedImages);
        setTotalResults(count);
        setKnownTags((prev) => mergeSuggestions(prev, buildSuggestionsFromImages(fetchedImages)));
      } catch (err: any) {
        setError(err.message || 'An unknown error occurred.');
      } finally {
        setLoading(false);
      }
    };

    fetchImages();
  }, [tagsQuery, pageSize, currentPage, sortBy, sortOrder]);

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
      updateUrlParams(query);
    };

    if (immediate) {
      if (debounceRef.current) window.clearTimeout(debounceRef.current);
      updateUrl();
      return;
    }

    if (debounceRef.current) window.clearTimeout(debounceRef.current);
    debounceRef.current = window.setTimeout(updateUrl, 350);
  };

  const updateSpecialFilter = (prefix: string, newValue: string | null) => {
    let tokens = tagsQuery.split(/\s+/).filter(Boolean);
    tokens = tokens.filter((t) => !t.startsWith(prefix));

    if (newValue) tokens.push(newValue);

    const newQuery = tokens.join(' ');

    if (debounceRef.current) window.clearTimeout(debounceRef.current);
    debounceRef.current = window.setTimeout(() => {
      updateUrlParams(newQuery);
    }, 350);
  };

  const updateStatusFilter = (statusValue: 'organized' | 'unorganized' | null) => {
    let tokens = tagsQuery.split(/\s+/).filter(Boolean);
    tokens = tokens.filter((t) => t !== 'is:organized' && t !== 'is:unorganized' && t !== 'is:missing');

    if (statusValue) {
      tokens.push(`is:${statusValue}`);
    }

    const newQuery = tokens.join(' ');

    if (debounceRef.current) window.clearTimeout(debounceRef.current);
    debounceRef.current = window.setTimeout(() => {
      updateUrlParams(newQuery);
    }, 350);
  };

  const updateRangeFilter = (prefix: string, min: number, max: number, anyThreshold: number, multiplier = 1) => {
    let tokens = tagsQuery.split(/\s+/).filter(Boolean);
    tokens = tokens.filter((t) => !t.startsWith(`${prefix}:>=`) && !t.startsWith(`${prefix}:<=`));

    if (min > 0) tokens.push(`${prefix}:>=${Math.floor(min * multiplier)}`);
    if (max < anyThreshold) tokens.push(`${prefix}:<=${Math.floor(max * multiplier)}`);

    const newQuery = tokens.join(' ');
    if (debounceRef.current) window.clearTimeout(debounceRef.current);
    debounceRef.current = window.setTimeout(() => {
      updateUrlParams(newQuery);
    }, 350);
  };

  const fetchSavedPalettes = async () => {
    try {
      const res = await fetch('/api/palettes');
      if (res.ok) {
        const data = await res.json();
        setSavedPalettes(data || []);
      }
    } catch (err) {
      console.error('Failed to load saved palettes:', err);
    }
  };

  useEffect(() => {
    fetchSavedPalettes();
  }, []);

  const handleSavePalette = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newPaletteName.trim() || vibePalette.length === 0) return;

    try {
      const res = await fetch('/api/palettes', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: newPaletteName.trim(),
          colors: vibePalette,
        }),
      });
      if (res.ok) {
        setNewPaletteName('');
        setIsSavingPalette(false);
        fetchSavedPalettes();
      } else {
        const err = await res.json();
        alert(err.error || 'Failed to save palette');
      }
    } catch (err) {
      console.error('Error saving palette:', err);
    }
  };

  const [confirmingDeletePaletteId, setConfirmingDeletePaletteId] = useState<number | null>(null);

  const handleDeleteSavedPalette = async (id: number, e: React.MouseEvent) => {
    e.stopPropagation();
    if (confirmingDeletePaletteId !== id) {
      setConfirmingDeletePaletteId(id);
      return;
    }
    try {
      await fetch(`/api/palettes/${id}`, { method: 'DELETE' });
      fetchSavedPalettes();
    } catch (err) {
      console.error('Error deleting saved palette:', err);
    } finally {
      setConfirmingDeletePaletteId(null);
    }
  };

  const handleApplyPalette = (colors: string[]) => {
    setVibePalette(colors);
    setHasColor(colors.length > 0);

    let tokens = tagsQuery.split(/\s+/).filter(Boolean);
    tokens = tokens.filter((t) => !t.startsWith('palette:') && !t.startsWith('color:'));
    if (colors.length > 0) {
      tokens.push(`palette:${colors.join(',')}`);
    }
    const newQuery = tokens.join(' ');

    if (debounceRef.current) window.clearTimeout(debounceRef.current);
    debounceRef.current = window.setTimeout(() => {
      updateUrlParams(newQuery);
    }, 150);
  };

  const handleColorChange = (newColor: string) => {
    setColor(newColor);
  };

  const handleAddCurrentColorToPalette = () => {
    const updated = Array.from(new Set([...vibePalette, color]));
    handleApplyPalette(updated);
  };

  const handleRemoveColorFromPalette = (cToRemove: string) => {
    const updated = vibePalette.filter((c) => c !== cToRemove);
    handleApplyPalette(updated);
  };

  const handleClearColor = () => {
    handleApplyPalette([]);
  };

  const handleAddTag = (category: TagCategory, name: string) => {
    applyFilters(addTagPill(filters, createTagPill(category, name)));
  };

  const handleExcludeTag = (category: TagCategory, name: string) => {
    applyFilters(addTagPill(filters, createTagPill(category, name, true)));
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

      setImages((prev) => prev.filter((img) => !selectedIds.has(img.id)));
      setSelectedIds(new Set());
      setSelectionMode(false);
    } catch (err: any) {
      alert(err.message || 'An error occurred during deletion.');
    } finally {
      setIsDeleting(false);
    }
  };

  const handleExtractPaletteFromSelected = async () => {
    const ids = Array.from(selectedIds);
    if (ids.length === 0 || isExtractingPalette) return;

    setIsExtractingPalette(true);
    try {
      const res = await fetch('/api/palettes/extract', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids }),
      });
      if (res.ok) {
        const data = await res.json();
        if (data.colors && data.colors.length > 0) {
          handleApplyPalette(data.colors);
          setSelectionMode(false);
          setSelectedIds(new Set());
        } else {
          alert('Could not extract dominant colors from selected images.');
        }
      } else {
        const err = await res.json();
        alert(err.error || 'Failed to extract palette');
      }
    } catch (err) {
      console.error('Error extracting palette:', err);
      alert('Error extracting palette');
    } finally {
      setIsExtractingPalette(false);
    }
  };

  const sortedImages = images;

  const totalPages = Math.max(1, Math.ceil(totalResults / pageSize));

  const renderPaginationBar = () => {
    if (totalResults <= 0) return null;
    return (
      <div className="flex items-center justify-between py-2.5 px-4 bg-[#15151a]/80 border-b border-[#2a2a35] text-xs shrink-0">
        <div className="text-gray-400">
          Showing <span className="font-medium text-gray-200">{Math.min((currentPage - 1) * pageSize + 1, totalResults)}</span> to{' '}
          <span className="font-medium text-gray-200">{Math.min(currentPage * pageSize, totalResults)}</span> of{' '}
          <span className="font-medium text-gray-200">{totalResults}</span> images
        </div>
        <div className="flex items-center gap-1.5">
          <button
            type="button"
            onClick={() => {
              setCurrentPage(1);
              updateUrlParams(undefined, undefined, undefined, undefined, 1);
            }}
            disabled={currentPage <= 1 || pageSize <= 0 || loading}
            className="px-2 py-1 rounded bg-[#1c1c24] border border-[#2a2a35] text-gray-300 hover:border-[#60a5fa] hover:text-[#60a5fa] disabled:opacity-40 disabled:pointer-events-none transition-colors cursor-pointer disabled:cursor-not-allowed"
            title="First Page"
          >
            «
          </button>
          <button
            type="button"
            onClick={() => {
              const prev = Math.max(1, currentPage - 1);
              setCurrentPage(prev);
              updateUrlParams(undefined, undefined, undefined, undefined, prev);
            }}
            disabled={currentPage <= 1 || loading}
            className="px-2 py-1 rounded bg-[#1c1c24] border border-[#2a2a35] text-gray-300 hover:border-[#60a5fa] hover:text-[#60a5fa] disabled:opacity-40 disabled:pointer-events-none transition-colors"
            title="Previous Page"
          >
            &lt;
          </button>

          <div className="flex items-center gap-1 mx-1">
            <select
              value={currentPage}
              onChange={(e) => {
                const p = Number(e.target.value);
                setCurrentPage(p);
                updateUrlParams(undefined, undefined, undefined, undefined, p);
              }}
              disabled={pageSize <= 0 || loading}
              className="bg-[#1c1c24] border border-[#2a2a35] text-gray-200 font-medium rounded px-2 py-1 outline-none focus:border-[#60a5fa] cursor-pointer disabled:cursor-not-allowed"
            >
              {Array.from({ length: totalPages }, (_, i) => i + 1).map((p) => (
                <option key={p} value={p}>
                  Page {p} of {totalPages}
                </option>
              ))}
            </select>
          </div>

          <button
            type="button"
            onClick={() => {
              const next = Math.min(totalPages, currentPage + 1);
              setCurrentPage(next);
              updateUrlParams(undefined, undefined, undefined, undefined, next);
            }}
            disabled={currentPage >= totalPages || pageSize <= 0 || loading}
            className="px-2 py-1 rounded bg-[#1c1c24] border border-[#2a2a35] text-gray-300 hover:border-[#60a5fa] hover:text-[#60a5fa] disabled:opacity-40 disabled:pointer-events-none transition-colors cursor-pointer disabled:cursor-not-allowed"
            title="Next Page"
          >
            &gt;
          </button>
          <button
            type="button"
            onClick={() => {
              setCurrentPage(totalPages);
              updateUrlParams(undefined, undefined, undefined, undefined, totalPages);
            }}
            disabled={currentPage >= totalPages || pageSize <= 0 || loading}
            className="px-2 py-1 rounded bg-[#1c1c24] border border-[#2a2a35] text-gray-300 hover:border-[#60a5fa] hover:text-[#60a5fa] disabled:opacity-40 disabled:pointer-events-none transition-colors cursor-pointer disabled:cursor-not-allowed"
            title="Last Page"
          >
            »
          </button>
        </div>
      </div>
    );
  };

  const startQueue = () => {
    if (sortedImages.length === 0) return;
    const ids = sortedImages.map((img) => img.id);
    sessionStorage.setItem('ago_queue', JSON.stringify({ ids }));
  };

  const renderTagList = (tags: TagCount[], category: TagCategory) => {
    if (!tags.length) return null;

    const isExpanded = expandedCategories[category];
    const visibleTags = isExpanded ? tags : tags.slice(0, 5);

    // Build a set of active search tag names for quick lookup
    const activeTagNames = new Set(
      filters.tags.map((t) => t.searchToken.replace(/^-/, '')),
    );

    return (
      <React.Fragment key={category}>
        {visibleTags.map((tag) => {
          const isActive = activeTagNames.has(tag.name);
          const isExcluded = filters.tags.some(
            (t) => t.negated && t.searchToken === `-${tag.name}`,
          );

          return (
            <li key={`${category}:${tag.name}`} className="flex items-center gap-1 group/tag">
              <button
                type="button"
                onClick={() => handleAddTag(category, tag.name)}
                className={`flex flex-1 items-start text-[13px] hover:underline cursor-pointer text-left min-w-0 ${
                  isExcluded ? 'opacity-40 line-through' : ''
                }`}
              >
                <span className={`${CATEGORY_COLORS[category]} font-medium leading-tight flex-1 truncate`}>
                  {tag.name}
                </span>
                <span className="text-gray-500 ml-1 shrink-0">{tag.count}</span>
              </button>
              {!isExcluded && (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleExcludeTag(category, tag.name);
                  }}
                  className={`shrink-0 w-5 h-5 flex items-center justify-center rounded transition-all cursor-pointer ${
                    isActive
                      ? 'text-red-400/70 hover:text-red-300 hover:bg-red-500/15'
                      : 'text-gray-600 opacity-0 group-hover/tag:opacity-100 hover:text-red-400 hover:bg-red-500/10'
                  }`}
                  title={`Exclude ${tag.name}`}
                >
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" className="w-3.5 h-3.5">
                    <path d="M3.5 8a.75.75 0 0 1 .75-.75h7.5a.75.75 0 0 1 0 1.5h-7.5A.75.75 0 0 1 3.5 8Z" />
                  </svg>
                </button>
              )}
            </li>
          );
        })}
        {tags.length > 5 && (
          <li>
            <button
              type="button"
              onClick={() => toggleCategoryExpanded(category)}
              className="text-xs text-[#60a5fa] hover:text-[#93c5fd] transition-colors mt-0.5 mb-2 ml-1 cursor-pointer"
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
          <div className="p-4 border-b border-[#2a2a35] overflow-y-auto hide-scrollbar">
            <h2 className="font-bold text-gray-200 mb-2 text-sm">Search</h2>
            <SearchAutocomplete
              draftInput={draftInput}
              onDraftChange={setDraftInput}
              onAddTag={handleAddTag}
              onExcludeTag={handleExcludeTag}
              suggestions={suggestions}
            />
            <SearchTagPills tags={filters.tags} onRemove={handleRemoveTag} />
            <SearchFiltersPanel
              filters={filters}
              onChange={(next) => applyFilters(next, true)}
              onSliderChange={(next) => applyFilters(next, false)}
            />

            {/* --- FILE PROPERTIES --- */}
            <div className="mt-4 pt-4 border-t border-[#2a2a35]">

              <RenderDualSlider
                label="Width"
                min={0} max={10500} step={100}
                currentMin={widthRange[0]} currentMax={widthRange[1]}
                formatValue={(v) => (v >= 10500 ? 'Any' : `${v}px`)}
                onChange={(min, max) => {
                  setWidthRange([min, max]);
                  updateRangeFilter('width', min, max, 10500);
                }}
              />

              <RenderDualSlider
                label="Height"
                min={0} max={10500} step={100}
                currentMin={heightRange[0]} currentMax={heightRange[1]}
                formatValue={(v) => (v >= 10500 ? 'Any' : `${v}px`)}
                onChange={(min, max) => {
                  setHeightRange([min, max]);
                  updateRangeFilter('height', min, max, 10500);
                }}
              />

              <RenderDualSlider
                label="File Size"
                min={0} max={21} step={1}
                currentMin={sizeRange[0]} currentMax={sizeRange[1]}
                formatValue={(v) => (v >= 21 ? 'Any' : `${v}MB`)}
                onChange={(min, max) => {
                  setSizeRange([min, max]);
                  updateRangeFilter('size', min, max, 21, 1024 * 1024);
                }}
              />
            </div>

            {/* --- APPEARANCE FILTERS --- */}
            <div className="mt-4 pt-4 border-t border-[#2a2a35]">

              <div className="mb-4">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs font-semibold text-gray-400 uppercase tracking-wide">Palette Match</span>
                  {hasColor && (
                    <button onClick={handleClearColor} className="text-xs text-red-400 hover:text-red-300 transition-colors cursor-pointer">
                      Clear
                    </button>
                  )}
                </div>

                {/* Saved Palettes */}
                {savedPalettes.length > 0 && (
                  <div className="flex flex-wrap gap-1.5 mb-2.5">
                    {savedPalettes.map((p) => (
                      <div
                        key={p.id}
                        onClick={() => handleApplyPalette(p.colors)}
                        className="group cursor-pointer px-2 py-1 bg-[#1c1c24] hover:bg-[#2a2a35] border border-[#2a2a35] rounded text-[10px] text-gray-300 transition-colors flex items-center gap-1"
                      >
                        <span className="flex">
                          {p.colors.slice(0, 3).map((c) => (
                            <span
                              key={c}
                              className="w-2 h-2 rounded-full inline-block -ml-0.5 first:ml-0"
                              style={{ backgroundColor: c }}
                            />
                          ))}
                        </span>
                        <span>{p.name}</span>
                        {confirmingDeletePaletteId === p.id ? (
                          <span className="flex items-center gap-1 ml-0.5">
                            <button
                              type="button"
                              onClick={(e) => handleDeleteSavedPalette(p.id, e)}
                              className="text-[10px] text-red-400 hover:text-red-300 font-semibold transition-colors cursor-pointer"
                            >
                              Delete?
                            </button>
                            <button
                              type="button"
                              onClick={(e) => { e.stopPropagation(); setConfirmingDeletePaletteId(null); }}
                              className="text-[10px] text-gray-500 hover:text-gray-300 transition-colors cursor-pointer"
                            >
                              Cancel
                            </button>
                          </span>
                        ) : (
                          <button
                            type="button"
                            onClick={(e) => handleDeleteSavedPalette(p.id, e)}
                            className="text-gray-500 hover:text-red-400 ml-0.5 transition-colors cursor-pointer"
                            title="Delete saved palette"
                          >
                            ×
                          </button>
                        )}
                      </div>
                    ))}
                  </div>
                )}

                {/* Active Vibe Palette Swatches */}
                {vibePalette.length > 0 && (
                  <div className="flex flex-wrap gap-1.5 mb-2.5 p-2 bg-[#15151a] border border-[#2a2a35] rounded max-h-24 overflow-y-auto">
                    {vibePalette.map((c) => (
                      <button
                        key={c}
                        type="button"
                        onClick={() => handleRemoveColorFromPalette(c)}
                        className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-[#1c1c24] hover:bg-[#2a2a35] border border-[#2a2a35] hover:border-red-500/50 text-[10px] text-gray-300 hover:text-red-300 font-mono transition-all cursor-pointer"
                        title="Click to remove color"
                      >
                        <span className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: c }} />
                        <span>{c.toUpperCase()}</span>
                      </button>
                    ))}
                  </div>
                )}

                <div className="flex flex-wrap items-center gap-2">
                  <input
                    type="color"
                    value={color}
                    onChange={(e) => handleColorChange(e.target.value)}
                    className="w-8 h-8 rounded cursor-pointer bg-transparent border-0 p-0 shrink-0"
                    title="Select a color"
                  />
                  <button
                    type="button"
                    onClick={handleAddCurrentColorToPalette}
                    className="px-2.5 py-1.5 bg-[#2563eb] hover:bg-[#1d4ed8] text-white rounded text-xs font-medium transition-colors shrink-0 cursor-pointer"
                  >
                    Add Color
                  </button>

                  {vibePalette.length > 0 && !isSavingPalette && (
                    <button
                      type="button"
                      onClick={() => setIsSavingPalette(true)}
                      className="px-2.5 py-1.5 bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-200 border border-[#3e3957] rounded text-xs transition-colors shrink-0 flex items-center gap-1 cursor-pointer"
                      title="Save current palette"
                    >
                      Save Palette
                    </button>
                  )}
                </div>

                {isSavingPalette && (
                  <form onSubmit={handleSavePalette} className="mt-2.5 p-2 bg-[#15151a] border border-[#2a2a35] rounded flex items-center gap-2">
                    <input
                      type="text"
                      placeholder="Palette name..."
                      value={newPaletteName}
                      onChange={(e) => setNewPaletteName(e.target.value)}
                      className="flex-1 min-w-0 bg-[#1c1c24] border border-[#2a2a35] rounded px-2 py-1 text-xs text-gray-200 focus:outline-none focus:border-blue-500"
                      autoFocus
                    />
                    <button
                      type="submit"
                      disabled={!newPaletteName.trim()}
                      className="px-2.5 py-1 bg-[#2563eb] hover:bg-[#1d4ed8] disabled:opacity-50 text-white rounded text-xs font-medium transition-colors cursor-pointer disabled:cursor-not-allowed"
                    >
                      Save
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setIsSavingPalette(false);
                        setNewPaletteName('');
                      }}
                      className="px-2.5 py-1 bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 rounded text-xs transition-colors cursor-pointer"
                    >
                      Cancel
                    </button>
                  </form>
                )}
              </div>

              <RenderDualSlider
                label="Brightness"
                min={0} max={255} step={1}
                currentMin={brightness[0]} currentMax={brightness[1]}
                formatValue={(v) => `${v}`}
                onChange={(min, max) => {
                  setBrightness([min, max]);
                  if (min === 0 && max === 255) {
                    updateSpecialFilter('brightness:', null);
                  } else {
                    updateSpecialFilter('brightness:', `brightness:${min}-${max}`);
                  }
                }}
              />
            </div>

            {/* --- STATUS FILTERS --- */}
            <div className="mt-4 pt-4 border-t border-[#2a2a35]">
              <h3 className="font-bold text-gray-400 mb-2 text-xs uppercase tracking-wider">Status</h3>
              <div className="flex gap-1.5 flex-wrap">
                <button
                  type="button"
                  onClick={() => updateStatusFilter(null)}
                  className={`px-2.5 py-1 rounded text-xs transition-colors border cursor-pointer ${isAnyStatus
                    ? 'border-[#60a5fa] bg-[#60a5fa]/20 text-[#93c5fd]'
                    : 'border-[#2a2a35] bg-[#111115] text-gray-400 hover:text-gray-200'
                    }`}
                >
                  Any
                </button>
                <button
                  type="button"
                  onClick={() => updateStatusFilter('organized')}
                  className={`px-2.5 py-1 rounded text-xs transition-colors border cursor-pointer ${isOrganized
                    ? 'border-[#60a5fa] bg-[#60a5fa]/20 text-[#93c5fd]'
                    : 'border-[#2a2a35] bg-[#111115] text-gray-400 hover:text-gray-200'
                    }`}
                >
                  Organized
                </button>
                <button
                  type="button"
                  onClick={() => updateStatusFilter('unorganized')}
                  className={`px-2.5 py-1 rounded text-xs transition-colors border cursor-pointer ${isUnorganized
                    ? 'border-[#60a5fa] bg-[#60a5fa]/20 text-[#93c5fd]'
                    : 'border-[#2a2a35] bg-[#111115] text-gray-400 hover:text-gray-200'
                    }`}
                >
                  Unorganized
                </button>
              </div>

              <div className="mt-2.5">
                <button
                  type="button"
                  onClick={() => updateSpecialFilter('is:duplicate', isDuplicate ? null : 'is:duplicate')}
                  className={`px-2.5 py-1 rounded text-xs transition-colors border cursor-pointer ${isDuplicate
                    ? 'border-[#60a5fa] bg-[#60a5fa]/20 text-[#93c5fd]'
                    : 'border-[#2a2a35] bg-[#111115] text-gray-400 hover:text-gray-200'
                    }`}
                >
                  Duplicate
                </button>
              </div>
            </div>

          </div>

          <div className="flex-1 overflow-y-auto p-4 border-t border-[#2a2a35] hide-scrollbar">
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
            <div className="flex items-center gap-2 pr-4 border-r border-[#2a2a35]">
              <SavedFiltersDropdown
                currentQuery={tagsQuery}
                currentSortBy={sortBy}
                currentSortOrder={sortOrder}
                onLoadFilter={(query, newSortBy, newSortOrder) => {
                  if (!query) setDraftInput('');
                  setSortBy(newSortBy as any);
                  setSortOrder(newSortOrder as any);
                  updateUrlParams(query, newSortBy, newSortOrder);
                }}
              />
            </div>

            <div className="flex items-center gap-2 pr-4 border-r border-[#2a2a35]">
              <span className="text-gray-500 font-medium">Sort by:</span>
              <select
                value={sortBy}
                onChange={(e) => {
                  const val = e.target.value as any;
                  setSortBy(val);
                  updateUrlParams(undefined, val, sortOrder);
                }}
                className="bg-[#1c1c24] border border-[#2a2a35] text-gray-300 rounded px-2 py-1.5 outline-none focus:border-[#60a5fa] transition-colors cursor-pointer"
              >
                <option value="none">None (Default)</option>
                <option value="created_at">Date Added</option>
                <option value="id">File ID</option>
                <option value="file_size">File Size</option>
                <option value="dimensions">Dimensions</option>
                <option value="rating">Rating</option>
                <option value="random">Random (Shuffle)</option>
              </select>
              <button
                type="button"
                onClick={() => {
                  if (sortBy === 'random') {
                    setRandomSeed(Math.random());
                  } else {
                    const val = sortOrder === 'asc' ? 'desc' : 'asc';
                    setSortOrder(val);
                    updateUrlParams(undefined, sortBy, val);
                  }
                }}
                disabled={sortBy === 'none'}
                className={`w-7 h-7 flex items-center justify-center border rounded transition-colors ${sortBy === 'none'
                  ? 'bg-[#15151a] border-[#2a2a35] text-gray-600 cursor-not-allowed'
                  : 'bg-[#1c1c24] border-[#2a2a35] hover:border-[#60a5fa] hover:text-[#60a5fa] text-gray-300 cursor-pointer'
                  }`}
                title={
                  sortBy === 'none'
                    ? 'Sorting disabled'
                    : sortBy === 'random'
                      ? 'Reshuffle queue order'
                      : sortOrder === 'asc'
                        ? 'Ascending'
                        : 'Descending'
                }
              >
                {sortBy === 'random' ? '🔀' : sortOrder === 'asc' ? '↑' : '↓'}
              </button>
            </div>

            <div className="flex items-center gap-2 pr-4 border-r border-[#2a2a35]">
              <span className="text-gray-500 font-medium">Per page:</span>
              <select
                value={pageSize}
                onChange={(e) => {
                  const val = Number(e.target.value);
                  setPageSize(val);
                  setCurrentPage(1);
                  updateUrlParams(undefined, undefined, undefined, val, 1);
                }}
                className="bg-[#1c1c24] border border-[#2a2a35] text-gray-300 rounded px-2 py-1.5 outline-none focus:border-[#60a5fa] transition-colors cursor-pointer"
              >
                <option value={50}>50</option>
                <option value={100}>100</option>
                <option value={250}>250</option>
                <option value={500}>500</option>
                <option value={-1}>All</option>
              </select>
            </div>

            <div className="flex items-center gap-2 pr-4 border-r border-[#2a2a35]">
              <span className="text-gray-500 font-medium">Zoom:</span>
              <input
                type="range"
                min={220}
                max={680}
                step={10}
                value={zoomSize}
                onChange={(e) => setZoomSize(Number(e.target.value))}
                className="w-24 md:w-32 accent-[#60a5fa] bg-[#1c1c24] h-1.5 rounded-lg cursor-pointer"
                title={`Grid column width: ${zoomSize}px`}
              />
            </div>

            <div className="ml-auto flex items-center gap-2">
              {sortedImages.length > 0 && !selectionMode && (
                <Link
                  to={`/image/${sortedImages[0].id}?queue=true`}
                  onClick={startQueue}
                  className="px-2.5 py-1.5 rounded border border-[#60a5fa]/60 text-[#60a5fa] bg-[#60a5fa]/10 hover:bg-[#60a5fa]/20 transition-colors flex items-center gap-1.5 text-xs font-medium cursor-pointer"
                  title="Start Queue with current search & sort results"
                >
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className="w-3.5 h-3.5">
                    <path d="M6.3 2.841A1.5 1.5 0 004 4.11V15.89a1.5 1.5 0 002.3 1.269l9.344-5.89a1.5 1.5 0 000-2.538L6.3 2.84z" />
                  </svg>
                  <span>Start Queue</span>
                </Link>
              )}

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
                  className="px-2.5 py-1.5 rounded border border-[#2a2a35] text-gray-400 hover:text-gray-200 transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50"
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
                className={`px-2.5 py-1.5 rounded border transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 ${selectionMode
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
                    onClick={handleExtractPaletteFromSelected}
                    disabled={isDeleting || isExtractingPalette}
                    className="px-2.5 py-1.5 rounded border border-purple-500/50 text-purple-300 hover:bg-purple-500/20 hover:text-purple-200 hover:border-purple-400 transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50 flex items-center gap-1.5"
                  >
                    {isExtractingPalette ? 'Extracting...' : `Extract Palette (${selectedImageIds.length})`}
                  </button>

                  <button
                    type="button"
                    onClick={() => setShowExportModal(true)}
                    disabled={isDeleting}
                    className="px-2.5 py-1.5 rounded border border-[#2a2a35] text-gray-300 hover:text-white hover:border-[#60a5fa] transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    Export ({selectedImageIds.length})
                  </button>

                  <button
                    type="button"
                    onClick={handleBatchDelete}
                    disabled={isDeleting}
                    className="px-2.5 py-1.5 rounded border border-red-500/50 text-red-400 hover:bg-red-500/20 hover:text-red-300 hover:border-red-500 transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {isDeleting ? 'Deleting...' : `Delete (${selectedImageIds.length})`}
                  </button>
                </>
              )}
            </div>
          </div>

          {renderPaginationBar()}

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
              <>
                <div
                  className="grid gap-4 content-start"
                  style={{
                    gridTemplateColumns: `repeat(auto-fill, minmax(${zoomSize}px, 1fr))`,
                  }}
                >
                  {sortedImages.map((img) => {
                    const isSelected = selectedIds.has(img.id);
                    return (
                      <div key={img.id} className="relative group w-full">
                        <Link
                          to={selectionMode ? '#' : `/image/${img.id}?queue=true`}
                          onClick={(event) => {
                            if (selectionMode) {
                              event.preventDefault();
                              if (!isDeleting) toggleSelected(img.id);
                            } else {
                              startQueue();
                            }
                          }}
                          className={`block relative rounded transition-all duration-200 ${
                            selectionMode && isSelected
                              ? 'ring-2 ring-[#60a5fa] bg-[#60a5fa]/10 scale-[0.98]'
                              : 'hover:ring-1 hover:ring-[#60a5fa]'
                          } ${isDeleting ? 'pointer-events-none' : ''}`}
                        >
                          <div
                            className={`bg-[#111115] p-1 rounded flex items-center justify-center overflow-hidden ${
                              selectionMode && isSelected ? 'opacity-80' : ''
                            }`}
                            style={{ height: `${zoomSize}px` }}
                          >
                            <img
                              src={`${img.thumbnail_path ? img.thumbnail_path : `/images/${img.file_name}`}?v=${img.file_size || ''}`}
                              alt={`Post ${img.id}`}
                              className="object-contain w-full h-full"
                              loading="lazy"
                            />
                          </div>

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
                <div className="mt-6 border-t border-[#2a2a35] pt-2">
                  {renderPaginationBar()}
                </div>
              </>
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
