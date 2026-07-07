import React from 'react';
import {
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
                className={`px-2.5 py-1 text-xs rounded border transition-colors ${active
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
                className={`px-2.5 py-1 text-xs rounded border transition-colors ${active
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
