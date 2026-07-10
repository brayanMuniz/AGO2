import React, { useEffect, useRef, useState } from 'react';
import {
  filterTagSuggestions,
  type TagCategory,
  type TagSuggestion,
} from '../utils/searchTags';

interface SearchAutocompleteProps {
  draftInput: string;
  onDraftChange: (value: string) => void;
  onAddTag: (category: TagCategory, name: string) => void;
  onExcludeTag: (category: TagCategory, name: string) => void;
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
  draftInput,
  onDraftChange,
  onAddTag,
  onExcludeTag,
  suggestions,
}) => {
  const [open, setOpen] = useState(false);
  const [highlightIndex, setHighlightIndex] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);

  const filtered = filterTagSuggestions(suggestions, draftInput.trim());

  useEffect(() => {
    setHighlightIndex(0);
  }, [draftInput, filtered.length]);

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
    onAddTag(suggestion.category, suggestion.name);
    onDraftChange('');
    setOpen(false);
  };

  const excludeSuggestion = (suggestion: TagSuggestion) => {
    onExcludeTag(suggestion.category, suggestion.name);
    onDraftChange('');
    setOpen(false);
  };

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (open && filtered.length > 0) {
      applySuggestion(filtered[highlightIndex]);
      return;
    }

    const trimmed = draftInput.trim();
    if (trimmed) {
      onAddTag('general', trimmed);
      onDraftChange('');
    }
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
          value={draftInput}
          onChange={(event) => {
            onDraftChange(event.target.value);
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

      {open && draftInput.trim() && filtered.length > 0 && (
        <ul className="absolute left-0 right-0 top-full z-20 mt-1 max-h-64 overflow-y-auto rounded-sm border border-[#2a2a35] bg-[#1c1c24] shadow-lg">
          {filtered.map((suggestion, index) => {
            const isActive = index === highlightIndex;

            return (
              <li key={`${suggestion.category}:${suggestion.name}`} className="flex items-center">
                <button
                  type="button"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => applySuggestion(suggestion)}
                  onMouseEnter={() => setHighlightIndex(index)}
                  className={`flex flex-1 items-center px-3 py-2 text-left text-sm min-w-0 ${
                    isActive ? 'bg-[#2a2a35]' : 'hover:bg-[#252530]'
                  }`}
                >
                  <span className="truncate">
                    <span className={`${CATEGORY_COLORS[suggestion.category]} font-medium`}>
                      {suggestion.category}:
                    </span>
                    <span className="text-gray-200">{suggestion.name}</span>
                  </span>
                  {suggestion.count != null && (
                    <span className="ml-auto pl-2 text-xs text-gray-500 shrink-0">{suggestion.count}</span>
                  )}
                </button>
                <button
                  type="button"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => excludeSuggestion(suggestion)}
                  className="shrink-0 w-8 h-full flex items-center justify-center text-gray-600 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                  title={`Exclude ${suggestion.name}`}
                >
                  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" className="w-3.5 h-3.5">
                    <path d="M3.5 8a.75.75 0 0 1 .75-.75h7.5a.75.75 0 0 1 0 1.5h-7.5A.75.75 0 0 1 3.5 8Z" />
                  </svg>
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
