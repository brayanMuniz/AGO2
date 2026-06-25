import type { TagCategory } from './searchTags';

export interface TagPill {
  id: string;
  label: string;
  searchToken: string;
}

export interface SearchFilters {
  tags: TagPill[];
  ratings: string[];
  minWidth: number;
  minHeight: number;
  minFileSize: number;
  isFavorite: boolean | null;
}

export const RATING_OPTIONS = [
  { value: 'g', label: 'G' },
  { value: 's', label: 'S' },
  { value: 'q', label: 'Q' },
  { value: 'e', label: 'E' },
] as const;

export const EMPTY_FILTERS: SearchFilters = {
  tags: [],
  ratings: [],
  minWidth: 0,
  minHeight: 0,
  minFileSize: 0,
  isFavorite: null,
};

function parseNumericFilter(token: string, prefix: string): number {
  const value = token.slice(prefix.length);
  if (value.startsWith('>=')) return Number.parseInt(value.slice(2), 10) || 0;
  if (value.startsWith('<=')) return 0;
  if (value.startsWith('>')) return Number.parseInt(value.slice(1), 10) || 0;
  if (value.startsWith('<')) return 0;
  return Number.parseInt(value, 10) || 0;
}

export function buildSearchQuery(filters: SearchFilters): string {
  const tokens: string[] = filters.tags.map((tag) => tag.searchToken);

  filters.ratings.forEach((rating) => {
    tokens.push(`rating:${rating}`);
  });

  if (filters.minWidth > 0) {
    tokens.push(`width:>=${filters.minWidth}`);
  }
  if (filters.minHeight > 0) {
    tokens.push(`height:>=${filters.minHeight}`);
  }
  if (filters.minFileSize > 0) {
    tokens.push(`size:>=${filters.minFileSize}`);
  }
  if (filters.isFavorite !== null) {
    tokens.push(`favorite:${filters.isFavorite}`);
  }

  return tokens.join(' ');
}

export function parseSearchQuery(query: string): SearchFilters {
  const filters: SearchFilters = {
    tags: [],
    ratings: [],
    minWidth: 0,
    minHeight: 0,
    minFileSize: 0,
    isFavorite: null,
  };

  query
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .forEach((token, index) => {
      const lower = token.toLowerCase();

      if (lower.startsWith('rating:')) {
        filters.ratings.push(lower.slice('rating:'.length));
        return;
      }
      if (lower.startsWith('width:')) {
        filters.minWidth = Math.max(filters.minWidth, parseNumericFilter(lower, 'width:'));
        return;
      }
      if (lower.startsWith('height:')) {
        filters.minHeight = Math.max(filters.minHeight, parseNumericFilter(lower, 'height:'));
        return;
      }
      if (lower.startsWith('size:')) {
        filters.minFileSize = Math.max(filters.minFileSize, parseNumericFilter(lower, 'size:'));
        return;
      }
      if (lower.startsWith('favorite:')) {
        filters.isFavorite = lower.slice('favorite:'.length) === 'true';
        return;
      }

      filters.tags.push({
        id: `tag-${index}-${token}`,
        label: token,
        searchToken: token,
      });
    });

  return filters;
}

export function createTagPill(category: TagCategory, name: string): TagPill {
  return {
    id: `${category}:${name}`,
    label: `${category}:${name}`,
    searchToken: name,
  };
}

export function removeTagPill(filters: SearchFilters, pillId: string): SearchFilters {
  return {
    ...filters,
    tags: filters.tags.filter((tag) => tag.id !== pillId),
  };
}

export function addTagPill(filters: SearchFilters, pill: TagPill): SearchFilters {
  if (filters.tags.some((tag) => tag.searchToken === pill.searchToken)) {
    return filters;
  }

  return {
    ...filters,
    tags: [...filters.tags, pill],
  };
}

export function formatFileSize(bytes: number): string {
  if (bytes >= 1_000_000) return `${(bytes / 1_000_000).toFixed(1)} MB`;
  if (bytes >= 1_000) return `${(bytes / 1_000).toFixed(0)} KB`;
  return `${bytes} B`;
}
