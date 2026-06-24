import React, { useEffect, useRef, useState } from 'react';
import {
  filterTagSuggestions,
  formatTagToken,
  getCurrentToken,
  replaceCurrentToken,
  toSearchQuery,
  type TagSuggestion,
} from '../utils/searchTags';

interface SearchAutocompleteProps {
  value: string;
  onChange: (value: string) => void;
  onSearch: (displayQuery: string) => void;
  suggestions: TagSuggestion[];
}

const CATEGORY_COLORS: Record<string, string> = {
  artist: 'text-[#fca5a5]',
  character: 'text-[#4ade80]',
  copyright: 'text-[#c084fc]',
  general: 'text-[#60a5fa]',
  meta: 'text-[#fb923c]',
};

const SearchAutocomplete: React.FC<SearchAutocompleteProps> = ({
  value,
  onChange,
  onSearch,
  suggestions,
}) => {
  const [open, setOpen] = useState(false);
  const [highlightIndex, setHighlightIndex] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);

  const currentToken = getCurrentToken(value);
  const filtered = filterTagSuggestions(suggestions, currentToken);

  useEffect(() => {
    setHighlightIndex(0);
  }, [currentToken, filtered.length]);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const applySuggestion = (suggestion: TagSuggestion) => {
    const token = formatTagToken(suggestion.category, suggestion.name);
    const nextValue = replaceCurrentToken(value, token);
    onChange(nextValue);
    onSearch(nextValue);
    setOpen(false);
  };

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (open && filtered.length > 0) {
      applySuggestion(filtered[highlightIndex]);
      return;
    }
    onSearch(value);
    setOpen(false);
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (!open || filtered.length === 0) return;

    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setHighlightIndex((prev) => (prev + 1) % filtered.length);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setHighlightIndex((prev) => (prev - 1 + filtered.length) % filtered.length);
    } else if (event.key === 'Escape') {
      setOpen(false);
    }
  };

  return (
    <div ref={containerRef} className="relative flex flex-1">
      <form onSubmit={handleSubmit} className="flex flex-1">
        <input
          type="text"
          value={value}
          onChange={(event) => {
            onChange(event.target.value);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={handleKeyDown}
          className="flex-1 bg-[#2a2a35] text-white px-2 py-1 text-sm border border-transparent focus:border-blue-500 focus:outline-none rounded-l-sm"
          placeholder="Search tags..."
          autoComplete="off"
          spellCheck={false}
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

      {open && currentToken && filtered.length > 0 && (
        <ul className="absolute left-0 right-0 top-full z-20 mt-1 max-h-64 overflow-y-auto rounded-sm border border-[#2a2a35] bg-[#1c1c24] shadow-lg">
          {filtered.map((suggestion, index) => {
            const token = formatTagToken(suggestion.category, suggestion.name);
            const searchToken = toSearchQuery(token);
            const isActive = index === highlightIndex;

            return (
              <li key={`${suggestion.category}:${suggestion.name}`}>
                <button
                  type="button"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => applySuggestion(suggestion)}
                  onMouseEnter={() => setHighlightIndex(index)}
                  className={`flex w-full items-center justify-between px-3 py-2 text-left text-sm ${
                    isActive ? 'bg-[#2a2a35]' : 'hover:bg-[#252530]'
                  }`}
                >
                  <span>
                    <span className={`${CATEGORY_COLORS[suggestion.category]} font-medium`}>
                      {suggestion.category}:
                    </span>
                    <span className="text-gray-200">{suggestion.name}</span>
                  </span>
                  <span className="ml-3 text-xs text-gray-500">{searchToken}</span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
};

export default SearchAutocomplete;
