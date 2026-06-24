export const TAG_CATEGORIES = {
  tags_artist: 'artist',
  tags_character: 'character',
  tags_copyright: 'copyright',
  tags_general: 'general',
  tags_meta: 'meta',
} as const;

export type TagCategoryKey = keyof typeof TAG_CATEGORIES;
export type TagCategory = (typeof TAG_CATEGORIES)[TagCategoryKey];

export interface TagSuggestion {
  name: string;
  category: TagCategory;
  count?: number;
}

export function formatTagToken(category: TagCategory, name: string): string {
  return `${category}:${name}`;
}

export function toSearchQuery(displayQuery: string): string {
  return displayQuery
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .map((token) => {
      const colonIdx = token.indexOf(':');
      if (colonIdx > 0) {
        return token.slice(colonIdx + 1);
      }
      return token;
    })
    .join(' ');
}

export function getCurrentToken(input: string): string {
  const trimmed = input.trimEnd();
  const lastSpace = trimmed.lastIndexOf(' ');
  return lastSpace === -1 ? trimmed : trimmed.slice(lastSpace + 1);
}

export function replaceCurrentToken(input: string, replacement: string): string {
  const trimmed = input.trimEnd();
  const lastSpace = trimmed.lastIndexOf(' ');
  if (lastSpace === -1) {
    return replacement;
  }
  return `${trimmed.slice(0, lastSpace + 1)}${replacement}`;
}

export function appendTagToQuery(currentInput: string, category: TagCategory, tagName: string): string {
  const token = formatTagToken(category, tagName);
  const trimmed = currentInput.trim();
  return trimmed ? `${trimmed} ${token}` : token;
}

export function filterTagSuggestions(
  suggestions: TagSuggestion[],
  query: string,
): TagSuggestion[] {
  const needle = query.toLowerCase();
  if (!needle) return [];

  return suggestions
    .filter((tag) => {
      const prefixed = formatTagToken(tag.category, tag.name).toLowerCase();
      return tag.name.toLowerCase().includes(needle) || prefixed.includes(needle);
    })
    .sort((a, b) => {
      const aName = a.name.toLowerCase();
      const bName = b.name.toLowerCase();
      const aStarts = aName.startsWith(needle) ? 0 : 1;
      const bStarts = bName.startsWith(needle) ? 0 : 1;
      if (aStarts !== bStarts) return aStarts - bStarts;
      return (b.count ?? 0) - (a.count ?? 0);
    })
    .slice(0, 12);
}
