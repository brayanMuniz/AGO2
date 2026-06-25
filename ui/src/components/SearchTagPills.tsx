import React from 'react';
import type { TagPill } from '../utils/searchQuery';

interface SearchTagPillsProps {
  tags: TagPill[];
  onRemove: (pillId: string) => void;
}

const SearchTagPills: React.FC<SearchTagPillsProps> = ({ tags, onRemove }) => {
  if (tags.length === 0) return null;

  return (
    <div className="flex flex-wrap gap-1.5 mt-2">
      {tags.map((tag) => (
        <button
          key={tag.id}
          type="button"
          onClick={() => onRemove(tag.id)}
          className="inline-flex items-center gap-1 rounded-full bg-[#2a2a35] px-2.5 py-1 text-xs text-gray-200 hover:bg-[#3a3a45] transition-colors"
          title="Click to remove"
        >
          <span>{tag.label}</span>
          <span className="text-gray-500">×</span>
        </button>
      ))}
    </div>
  );
};

export default SearchTagPills;
