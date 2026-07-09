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
      {tags.map((tag) => (
        <button
          key={tag.id}
          type="button"
          onClick={() => onRemove(tag.id)}
          className="max-w-[200px] inline-flex items-center gap-1.5 rounded-full bg-[#2a2a35] px-2.5 py-1 text-xs text-gray-200 hover:bg-[#3a3a45] transition-colors"
          title={`${tag.label} (Click to remove)`}
        >
          {renderPillContent(tag.label)}
          <span className="text-gray-500 shrink-0">×</span>
        </button>
      ))}
    </div>
  );
};

export default SearchTagPills;
