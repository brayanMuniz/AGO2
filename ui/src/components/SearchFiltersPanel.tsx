import React from 'react';
import {
  formatFileSize,
  RATING_OPTIONS,
  type SearchFilters,
} from '../utils/searchQuery';

interface SearchFiltersPanelProps {
  filters: SearchFilters;
  onChange: (filters: SearchFilters) => void;
  onSliderChange: (filters: SearchFilters) => void;
}

const SearchFiltersPanel: React.FC<SearchFiltersPanelProps> = ({
  filters,
  onChange,
  onSliderChange,
}) => {
  const update = (partial: Partial<SearchFilters>) => {
    onChange({ ...filters, ...partial });
  };

  const toggleRating = (rating: string) => {
    const ratings = filters.ratings.includes(rating)
      ? filters.ratings.filter((value) => value !== rating)
      : [...filters.ratings, rating];
    update({ ratings });
  };

  return (
    <div className="mt-4 space-y-4 border-t border-[#2a2a35] pt-4">
      <div>
        <h3 className="text-xs font-semibold text-gray-400 mb-2 uppercase tracking-wide">Rating</h3>
        <div className="flex flex-wrap gap-1.5">
          {RATING_OPTIONS.map((option) => {
            const active = filters.ratings.includes(option.value);
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => toggleRating(option.value)}
                className={`px-2.5 py-1 text-xs rounded border transition-colors ${
                  active
                    ? 'border-[#60a5fa] bg-[#60a5fa]/20 text-[#93c5fd]'
                    : 'border-[#2a2a35] bg-[#111115] text-gray-400 hover:text-gray-200'
                }`}
              >
                {option.label}
              </button>
            );
          })}
        </div>
      </div>

      <div>
        <div className="flex items-center justify-between mb-1">
          <label htmlFor="min-width" className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
            Min width
          </label>
          <span className="text-xs text-gray-500">
            {filters.minWidth > 0 ? `${filters.minWidth}px` : 'Any'}
          </span>
        </div>
        <input
          id="min-width"
          type="range"
          min={0}
          max={8000}
          step={100}
          value={filters.minWidth}
          onChange={(event) => onSliderChange({ ...filters, minWidth: Number(event.target.value) })}
          className="w-full accent-[#60a5fa]"
        />
      </div>

      <div>
        <div className="flex items-center justify-between mb-1">
          <label htmlFor="min-height" className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
            Min height
          </label>
          <span className="text-xs text-gray-500">
            {filters.minHeight > 0 ? `${filters.minHeight}px` : 'Any'}
          </span>
        </div>
        <input
          id="min-height"
          type="range"
          min={0}
          max={8000}
          step={100}
          value={filters.minHeight}
          onChange={(event) => onSliderChange({ ...filters, minHeight: Number(event.target.value) })}
          className="w-full accent-[#60a5fa]"
        />
      </div>

      <div>
        <div className="flex items-center justify-between mb-1">
          <label htmlFor="min-file-size" className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
            Min file size
          </label>
          <span className="text-xs text-gray-500">
            {filters.minFileSize > 0 ? formatFileSize(filters.minFileSize) : 'Any'}
          </span>
        </div>
        <input
          id="min-file-size"
          type="range"
          min={0}
          max={20_000_000}
          step={100_000}
          value={filters.minFileSize}
          onChange={(event) => onSliderChange({ ...filters, minFileSize: Number(event.target.value) })}
          className="w-full accent-[#60a5fa]"
        />
      </div>

      <div>
        <h3 className="text-xs font-semibold text-gray-400 mb-2 uppercase tracking-wide">Favorites</h3>
        <div className="flex gap-1.5">
          {[
            { label: 'All', value: null },
            { label: 'Favorites', value: true },
            { label: 'Not favorites', value: false },
          ].map((option) => {
            const active = filters.isFavorite === option.value;
            return (
              <button
                key={option.label}
                type="button"
                onClick={() => update({ isFavorite: option.value })}
                className={`px-2.5 py-1 text-xs rounded border transition-colors ${
                  active
                    ? 'border-[#60a5fa] bg-[#60a5fa]/20 text-[#93c5fd]'
                    : 'border-[#2a2a35] bg-[#111115] text-gray-400 hover:text-gray-200'
                }`}
              >
                {option.label}
              </button>
            );
          })}
        </div>
      </div>
    </div>
  );
};

export default SearchFiltersPanel;
