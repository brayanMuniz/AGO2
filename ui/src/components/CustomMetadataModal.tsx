import React, { useState, useEffect, useRef } from 'react';

export interface CustomPostData {
  id?: number;
  rating: string;
  source: string;
  tags_artist: string[];
  tags_copyright: string[];
  tags_character: string[];
  tags_general: string[];
  tags_meta: string[];
}

interface TagSuggestion {
  name: string;
  category: string;
  count?: number;
}

interface CustomMetadataModalProps {
  imageId: number;
  fileName: string;
  initialData?: {
    rating?: string;
    source?: string;
    tags_artist?: string[];
    tags_copyright?: string[];
    tags_character?: string[];
    tags_general?: string[];
    tags_meta?: string[];
  };
  onClose: () => void;
  onSaved: () => void;
}

type CategoryKey = 'artist' | 'copyright' | 'character' | 'general' | 'meta';

const CATEGORIES: { key: CategoryKey; label: string; color: string; border: string; bg: string }[] = [
  { key: 'artist', label: 'Artist', color: 'text-[#fca5a5]', border: 'border-[#fca5a5]/30', bg: 'bg-[#fca5a5]/10' },
  { key: 'copyright', label: 'Copyright', color: 'text-[#c084fc]', border: 'border-[#c084fc]/30', bg: 'bg-[#c084fc]/10' },
  { key: 'character', label: 'Character', color: 'text-[#4ade80]', border: 'border-[#4ade80]/30', bg: 'bg-[#4ade80]/10' },
  { key: 'general', label: 'General', color: 'text-[#60a5fa]', border: 'border-[#60a5fa]/30', bg: 'bg-[#60a5fa]/10' },
  { key: 'meta', label: 'Meta', color: 'text-[#fb923c]', border: 'border-[#fb923c]/30', bg: 'bg-[#fb923c]/10' },
];

export const CustomMetadataModal: React.FC<CustomMetadataModalProps> = ({
  imageId,
  fileName,
  initialData,
  onClose,
  onSaved,
}) => {
  const [rating, setRating] = useState<string>(initialData?.rating || 'g');
  const [tags, setTags] = useState<Record<CategoryKey, string[]>>({
    artist: initialData?.tags_artist || [],
    copyright: initialData?.tags_copyright || [],
    character: initialData?.tags_character || [],
    general: initialData?.tags_general || [],
    meta: initialData?.tags_meta || [],
  });

  const [inputVal, setInputVal] = useState<Record<CategoryKey, string>>({
    artist: '',
    copyright: '',
    character: '',
    general: '',
    meta: '',
  });

  const [activeCategory, setActiveCategory] = useState<CategoryKey | null>(null);
  const [suggestions, setSuggestions] = useState<TagSuggestion[]>([]);
  const [saving, setSaving] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!activeCategory) {
      setSuggestions([]);
      return;
    }

    const query = inputVal[activeCategory].trim();

    const timer = setTimeout(async () => {
      try {
        const res = await fetch(
          `/api/tags/autocomplete?category=${activeCategory}&query=${encodeURIComponent(query)}`
        );
        if (res.ok) {
          const data: TagSuggestion[] = await res.json();
          setSuggestions(data || []);
        }
      } catch {
        setSuggestions([]);
      }
    }, 150);

    return () => clearTimeout(timer);
  }, [activeCategory, inputVal]);

  useEffect(() => {
    const handleMouseDown = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (!target.closest('.tag-autocomplete-box')) {
        setActiveCategory(null);
      }
    };
    document.addEventListener('mousedown', handleMouseDown);
    return () => document.removeEventListener('mousedown', handleMouseDown);
  }, []);

  const handleAddTag = (category: CategoryKey, rawName: string) => {
    const cleaned = rawName
      .trim()
      .toLowerCase()
      .replace(/\s+/g, '_');

    if (!cleaned) return;

    setTags((prev) => {
      if (prev[category].includes(cleaned)) return prev;
      return { ...prev, [category]: [...prev[category], cleaned] };
    });

    setInputVal((prev) => ({ ...prev, [category]: '' }));
    setSuggestions([]);
  };

  const handleRemoveTag = (category: CategoryKey, tagToRemove: string) => {
    setTags((prev) => ({
      ...prev,
      [category]: prev[category].filter((t) => t !== tagToRemove),
    }));
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);

    const postPayload = {
      id: 0,
      source: 'Custom',
      rating,
      tags_artist: tags.artist,
      tags_copyright: tags.copyright,
      tags_character: tags.character,
      tags_general: tags.general,
      tags_meta: tags.meta,
    };

    try {
      const res = await fetch(`/api/image/${imageId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          main_data: postPayload,
        }),
      });

      if (!res.ok) {
        const errJson = await res.json();
        throw new Error(errJson.error || 'Failed to save custom metadata');
      }

      onSaved();
    } catch (err: any) {
      setError(err.message || 'An error occurred while saving.');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div
        ref={containerRef}
        className="bg-[#1c1c24] border border-[#2a2a35] rounded-2xl w-full max-w-2xl max-h-[90vh] flex flex-col shadow-2xl overflow-hidden"
      >
        {/* Header */}
        <div className="p-5 border-b border-[#2a2a35] flex items-center justify-between bg-[#15151a]">
          <div>
            <h2 className="text-lg font-bold text-gray-100 flex items-center gap-2">
              <span className="px-2 py-0.5 bg-purple-500/20 text-purple-400 text-xs rounded font-semibold border border-purple-500/30">
                Custom Metadata
              </span>
              <span>{fileName}</span>
            </h2>
            <p className="text-xs text-gray-400 mt-1">
              Add or edit tags by category and configure rating. Source will be saved as Custom.
            </p>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-white p-2 rounded-lg hover:bg-[#2a2a35] transition-colors cursor-pointer"
          >
            ✕
          </button>
        </div>

        {/* Content */}
        <div className="p-6 overflow-y-auto flex-1 space-y-6 hide-scrollbar">
          {error && (
            <div className="p-3 bg-red-500/20 border border-red-500/40 text-red-300 rounded-lg text-sm">
              {error}
            </div>
          )}

          {/* Rating selector */}
          <div>
            <label className="block text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2">
              Rating
            </label>
            <div className="flex gap-2">
              {[
                { val: 'g', label: 'General' },
                { val: 's', label: 'Sensitive' },
                { val: 'q', label: 'Questionable' },
                { val: 'e', label: 'Explicit' },
              ].map((r) => (
                <button
                  key={r.val}
                  type="button"
                  onClick={() => setRating(r.val)}
                  className={`px-4 py-2 rounded-lg text-xs font-semibold transition-all border cursor-pointer ${
                    rating === r.val
                      ? 'bg-[#60a5fa] text-white border-[#60a5fa] shadow-lg shadow-[#60a5fa]/20'
                      : 'bg-[#15151a] text-gray-400 border-[#2a2a35] hover:border-gray-500'
                  }`}
                >
                  {r.label}
                </button>
              ))}
            </div>
          </div>

          {/* Categories */}
          <div className="space-y-5">
            {CATEGORIES.map((cat) => {
              const catTags = tags[cat.key];
              const isFocused = activeCategory === cat.key;

              return (
                <div key={cat.key} className="space-y-2">
                  <div className="flex items-center justify-between">
                    <span className={`text-xs font-bold uppercase tracking-wider ${cat.color}`}>
                      {cat.label} ({catTags.length})
                    </span>
                  </div>

                  {/* Existing Tags */}
                  {catTags.length > 0 && (
                    <div className="flex flex-wrap gap-1.5">
                      {catTags.map((tag) => (
                        <span
                          key={tag}
                          className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium border ${cat.border} ${cat.bg} ${cat.color}`}
                        >
                          <span>{tag}</span>
                          <button
                            type="button"
                            onClick={() => handleRemoveTag(cat.key, tag)}
                            className="hover:text-white transition-colors cursor-pointer"
                            title="Remove tag"
                          >
                            ×
                          </button>
                        </span>
                      ))}
                    </div>
                  )}

                  {/* Input + Autocomplete dropdown */}
                  <div className="relative tag-autocomplete-box">
                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        handleAddTag(cat.key, inputVal[cat.key]);
                      }}
                      className="flex gap-2"
                    >
                      <input
                        type="text"
                        placeholder={`Add ${cat.label.toLowerCase()} tag...`}
                        value={inputVal[cat.key]}
                        onFocus={() => setActiveCategory(cat.key)}
                        onChange={(e) =>
                          setInputVal((prev) => ({ ...prev, [cat.key]: e.target.value }))
                        }
                        className="flex-1 bg-[#15151a] border border-[#2a2a35] rounded-lg px-3 py-2 text-sm text-gray-200 placeholder-gray-500 focus:outline-none focus:border-[#60a5fa]"
                      />
                      <button
                        type="submit"
                        className="px-4 py-2 bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 rounded-lg text-xs font-semibold transition-colors cursor-pointer"
                      >
                        + Add
                      </button>
                    </form>

                    {/* Dropdown Suggestions */}
                    {isFocused && suggestions.length > 0 && (
                      <div className="absolute left-0 right-0 mt-1 bg-[#1c1c24] border border-[#2a2a35] rounded-lg shadow-xl max-h-48 overflow-y-auto z-30">
                        {suggestions.map((item) => (
                          <button
                            key={item.name}
                            type="button"
                            onClick={() => handleAddTag(cat.key, item.name)}
                            className="w-full text-left px-3 py-2 text-xs hover:bg-[#2a2a35] flex items-center justify-between transition-colors cursor-pointer"
                          >
                            <span className={cat.color}>{item.name}</span>
                            {item.count !== undefined && (
                              <span className="text-gray-500 font-mono text-[10px]">
                                {item.count}
                              </span>
                            )}
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-[#2a2a35] bg-[#15151a] flex items-center justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            disabled={saving}
            className="px-4 py-2 rounded-lg bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 text-xs font-semibold transition-colors cursor-pointer"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="px-5 py-2 rounded-lg bg-[#60a5fa] hover:bg-[#3b82f6] text-white text-xs font-semibold transition-colors shadow-lg shadow-[#60a5fa]/20 cursor-pointer disabled:opacity-50"
          >
            {saving ? 'Saving...' : 'Save Custom Metadata'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default CustomMetadataModal;
