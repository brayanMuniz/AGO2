import React from 'react';
import type { TagPill } from '../utils/searchQuery';

interface SearchTagPillsProps {
  tags: TagPill[];
  onRemove: (pillId: string) => void;
}

const renderPillContent = (label: string) => {
  if (label.startsWith('palette:')) {
    const colors = label.slice(8).split(',').filter(Boolean);
    return (
      <span className="inline-flex items-center gap-1.5 truncate">
        <span className="inline-flex items-center -space-x-1 shrink-0">
          {colors.slice(0, 4).map((c, i) => (
            <span
              key={i}
              className="w-2.5 h-2.5 rounded-full border border-[#2a2a35] inline-block"
              style={{ backgroundColor: c }}
            />
          ))}
        </span>
        <span className="truncate">Palette ({colors.length})</span>
      </span>
    );
  }
  return <span className="truncate">{label}</span>;
};

const SearchTagPills: React.FC<SearchTagPillsProps> = ({ tags, onRemove }) => {
  if (tags.length === 0) return null;

  return (
    <div className="flex flex-wrap gap-1.5 mt-2">
      {tags.map((tag) => {
        const isNegated = tag.negated;
        return (
          <button
            key={tag.id}
            type="button"
            onClick={() => onRemove(tag.id)}
            className={`max-w-[200px] inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs transition-colors ${
              isNegated
                ? 'bg-red-500/15 text-red-300 hover:bg-red-500/25 border border-red-500/30'
                : 'bg-[#2a2a35] text-gray-200 hover:bg-[#3a3a45]'
            }`}
            title={`${isNegated ? `Excluding: ${tag.label}` : tag.label} (Click to remove)`}
          >
            {isNegated && (
              <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" fill="currentColor" className="w-3 h-3 shrink-0 text-red-400">
                <path d="M3.5 8a.75.75 0 0 1 .75-.75h7.5a.75.75 0 0 1 0 1.5h-7.5A.75.75 0 0 1 3.5 8Z" />
              </svg>
            )}
            {renderPillContent(tag.label)}
            <span className={`shrink-0 ${isNegated ? 'text-red-400' : 'text-gray-500'}`}>×</span>
          </button>
        );
      })}
    </div>
  );
};

export default SearchTagPills;
