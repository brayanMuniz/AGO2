import React, { useEffect, useRef, useState } from 'react';
import {
  getSavedFilters,
  createSavedFilter,
  updateSavedFilter,
  deleteSavedFilter,
  type SavedFilter,
} from '../api/filters';

interface SavedFiltersDropdownProps {
  currentQuery: string;
  onLoadFilter: (query: string) => void;
}

const SavedFiltersDropdown: React.FC<SavedFiltersDropdownProps> = ({
  currentQuery,
  onLoadFilter,
}) => {
  const [filters, setFilters] = useState<SavedFilter[]>([]);
  const [isOpen, setIsOpen] = useState(false);
  const [activeFilterId, setActiveFilterId] = useState<number | null>(null);
  const [isNaming, setIsNaming] = useState(false);
  const [newName, setNewName] = useState('');
  const [error, setError] = useState<string | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    fetchFilters();
  }, []);

  // Close dropdown on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
        setIsNaming(false);
        setError(null);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  // Auto-focus input when naming
  useEffect(() => {
    if (isNaming && inputRef.current) {
      inputRef.current.focus();
    }
  }, [isNaming]);

  const fetchFilters = async () => {
    try {
      const data = await getSavedFilters();
      setFilters(data);
    } catch {
      console.error('Failed to fetch saved filters');
    }
  };

  const handleSelect = (filter: SavedFilter) => {
    setActiveFilterId(filter.id);
    onLoadFilter(filter.query);
    setIsOpen(false);
    setIsNaming(false);
    setError(null);
  };

  const handleSaveNew = async () => {
    const trimmed = newName.trim();
    if (!trimmed) return;

    try {
      setError(null);
      const created = await createSavedFilter(trimmed, currentQuery);
      setFilters((prev) => [...prev, created].sort((a, b) => a.name.localeCompare(b.name)));
      setActiveFilterId(created.id);
      setIsNaming(false);
      setNewName('');
    } catch (err: any) {
      setError(err.message || 'Failed to save filter');
    }
  };

  const handleOverwrite = async (filter: SavedFilter, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      setError(null);
      const updated = await updateSavedFilter(filter.id, filter.name, currentQuery);
      setFilters((prev) => prev.map((f) => (f.id === updated.id ? updated : f)));
      setActiveFilterId(updated.id);
    } catch (err: any) {
      setError(err.message || 'Failed to update filter');
    }
  };

  const handleDelete = async (filter: SavedFilter, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      setError(null);
      await deleteSavedFilter(filter.id);
      setFilters((prev) => prev.filter((f) => f.id !== filter.id));
      if (activeFilterId === filter.id) {
        setActiveFilterId(null);
      }
    } catch (err: any) {
      setError(err.message || 'Failed to delete filter');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSaveNew();
    } else if (e.key === 'Escape') {
      setIsNaming(false);
      setNewName('');
      setError(null);
    }
  };

  const activeFilter = filters.find((f) => f.id === activeFilterId);
  const buttonLabel = activeFilter ? activeFilter.name : 'Saved Filters';

  return (
    <div className="relative" ref={dropdownRef}>
      {/* Trigger Button */}
      <button
        type="button"
        onClick={() => {
          setIsOpen((prev) => !prev);
          setIsNaming(false);
          setError(null);
        }}
        className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded border transition-colors text-xs cursor-pointer ${
          activeFilterId
            ? 'border-[#60a5fa] text-[#93c5fd] bg-[#60a5fa]/10'
            : 'border-[#2a2a35] text-gray-400 hover:text-gray-200 hover:border-gray-500'
        }`}
        title="Saved Filters"
      >
        {/* Bookmark icon */}
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 20 20"
          fill="currentColor"
          className="w-3.5 h-3.5"
        >
          <path
            fillRule="evenodd"
            d="M10 2c-1.716 0-3.408.106-5.07.31C3.806 2.45 3 3.414 3 4.517V17.25a.75.75 0 001.075.676L10 15.082l5.925 2.844A.75.75 0 0017 17.25V4.517c0-1.103-.806-2.068-1.93-2.207A41.403 41.403 0 0010 2z"
            clipRule="evenodd"
          />
        </svg>
        <span className="max-w-[120px] truncate">{buttonLabel}</span>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 20 20"
          fill="currentColor"
          className={`w-3.5 h-3.5 transition-transform ${isOpen ? 'rotate-180' : ''}`}
        >
          <path
            fillRule="evenodd"
            d="M5.22 8.22a.75.75 0 011.06 0L10 11.94l3.72-3.72a.75.75 0 111.06 1.06l-4.25 4.25a.75.75 0 01-1.06 0L5.22 9.28a.75.75 0 010-1.06z"
            clipRule="evenodd"
          />
        </svg>
      </button>

      {/* Dropdown Panel */}
      {isOpen && (
        <div className="absolute top-full left-0 mt-1 w-72 bg-[#1c1c24] border border-[#2a2a35] rounded-lg shadow-2xl z-50 overflow-hidden">
          {/* Header */}
          <div className="px-3 py-2 border-b border-[#2a2a35] flex items-center justify-between">
            <span className="text-xs font-semibold text-gray-400 uppercase tracking-wide">
              Saved Filters
            </span>
            {activeFilterId && (
              <button
                type="button"
                onClick={() => {
                  setActiveFilterId(null);
                }}
                className="text-[10px] text-red-400 hover:text-red-300 transition-colors"
              >
                Clear
              </button>
            )}
          </div>

          {/* Error message */}
          {error && (
            <div className="px-3 py-1.5 bg-red-500/10 border-b border-red-500/20">
              <p className="text-[11px] text-red-400">{error}</p>
            </div>
          )}

          {/* Filter list */}
          <div className="max-h-60 overflow-y-auto">
            {filters.length === 0 && !isNaming && (
              <div className="px-3 py-4 text-center text-xs text-gray-500">
                No saved filters yet
              </div>
            )}

            {filters.map((filter) => {
              const isActive = filter.id === activeFilterId;
              return (
                <div
                  key={filter.id}
                  onClick={() => handleSelect(filter)}
                  className={`group flex items-center gap-2 px-3 py-2 cursor-pointer transition-colors ${
                    isActive
                      ? 'bg-[#60a5fa]/10 text-[#93c5fd]'
                      : 'text-gray-300 hover:bg-[#25252f]'
                  }`}
                >
                  <span className="flex-1 text-xs truncate" title={filter.query}>
                    {filter.name}
                  </span>

                  <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    {/* Overwrite (save) button */}
                    <button
                      type="button"
                      onClick={(e) => handleOverwrite(filter, e)}
                      className="p-0.5 rounded hover:bg-[#60a5fa]/20 text-gray-500 hover:text-[#60a5fa] transition-colors"
                      title={`Overwrite "${filter.name}" with current filters`}
                    >
                      {/* Save/floppy disk icon */}
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        viewBox="0 0 20 20"
                        fill="currentColor"
                        className="w-3.5 h-3.5"
                      >
                        <path d="M3.196 12.87l-.825.483a.75.75 0 000 1.294l7.004 4.086a1.5 1.5 0 001.25 0l7.004-4.086a.75.75 0 000-1.294l-.825-.484-5.929 3.46a3 3 0 01-2.75 0L3.196 12.87z" />
                        <path d="M3.196 8.87l-.825.483a.75.75 0 000 1.294l7.004 4.086a1.5 1.5 0 001.25 0l7.004-4.086a.75.75 0 000-1.294l-.825-.484-5.929 3.46a3 3 0 01-2.75 0L3.196 8.87z" />
                        <path d="M10.625 2.247a1.5 1.5 0 00-1.25 0L2.371 6.333a.75.75 0 000 1.294l7.004 4.086a1.5 1.5 0 001.25 0l7.004-4.086a.75.75 0 000-1.294L10.625 2.247z" />
                      </svg>
                    </button>

                    {/* Delete button */}
                    <button
                      type="button"
                      onClick={(e) => handleDelete(filter, e)}
                      className="p-0.5 rounded hover:bg-red-500/20 text-gray-500 hover:text-red-400 transition-colors"
                      title={`Delete "${filter.name}"`}
                    >
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        viewBox="0 0 20 20"
                        fill="currentColor"
                        className="w-3.5 h-3.5"
                      >
                        <path
                          fillRule="evenodd"
                          d="M8.75 1A2.75 2.75 0 006 3.75v.443c-.795.077-1.584.176-2.365.298a.75.75 0 10.23 1.482l.149-.022.841 10.518A2.75 2.75 0 007.596 19h4.807a2.75 2.75 0 002.742-2.53l.841-10.52.149.023a.75.75 0 00.23-1.482A41.03 41.03 0 0014 4.193V3.75A2.75 2.75 0 0011.25 1h-2.5zM10 4c.84 0 1.673.025 2.5.075V3.75c0-.69-.56-1.25-1.25-1.25h-2.5c-.69 0-1.25.56-1.25 1.25v.325C8.327 4.025 9.16 4 10 4zM8.58 7.72a.75.75 0 01.78.72l.5 6.5a.75.75 0 01-1.499.115l-.5-6.5a.75.75 0 01.72-.78zm2.84 0a.75.75 0 01.72.78l-.5 6.5a.75.75 0 11-1.499-.115l.5-6.5a.75.75 0 01.78-.72z"
                          clipRule="evenodd"
                        />
                      </svg>
                    </button>
                  </div>
                </div>
              );
            })}
          </div>

          {/* Save New section */}
          <div className="border-t border-[#2a2a35] px-3 py-2">
            {isNaming ? (
              <div className="flex items-center gap-2">
                <input
                  ref={inputRef}
                  type="text"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Filter name..."
                  className="flex-1 bg-[#0e0e12] border border-[#2a2a35] rounded px-2 py-1 text-xs text-gray-200 outline-none focus:border-[#60a5fa] transition-colors placeholder:text-gray-600"
                />
                <button
                  type="button"
                  onClick={handleSaveNew}
                  disabled={!newName.trim()}
                  className="px-2 py-1 rounded bg-[#60a5fa]/20 text-[#60a5fa] text-xs hover:bg-[#60a5fa]/30 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  Save
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setIsNaming(false);
                    setNewName('');
                    setError(null);
                  }}
                  className="px-1.5 py-1 rounded text-gray-500 text-xs hover:text-gray-300 transition-colors"
                >
                  ✕
                </button>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setIsNaming(true)}
                disabled={!currentQuery.trim()}
                className="w-full text-xs text-[#60a5fa] hover:text-[#93c5fd] transition-colors py-1 disabled:text-gray-600 disabled:cursor-not-allowed"
              >
                + Save current filters
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default SavedFiltersDropdown;
