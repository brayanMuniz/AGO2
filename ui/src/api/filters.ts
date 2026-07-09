export interface SavedFilter {
  id: number;
  name: string;
  query: string;
  created_at: string;
  updated_at: string;
}

export async function getSavedFilters(): Promise<SavedFilter[]> {
  const response = await fetch('/api/filters');
  if (!response.ok) throw new Error('Failed to fetch saved filters');
  return response.json();
}

export async function createSavedFilter(name: string, query: string): Promise<SavedFilter> {
  const response = await fetch('/api/filters', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, query }),
  });
  if (!response.ok) {
    const err = await response.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to create saved filter');
  }
  return response.json();
}

export async function updateSavedFilter(id: number, name: string, query: string): Promise<SavedFilter> {
  const response = await fetch(`/api/filters/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, query }),
  });
  if (!response.ok) {
    const err = await response.json().catch(() => ({}));
    throw new Error(err.error || 'Failed to update saved filter');
  }
  return response.json();
}

export async function deleteSavedFilter(id: number): Promise<void> {
  const response = await fetch(`/api/filters/${id}`, { method: 'DELETE' });
  if (!response.ok) throw new Error('Failed to delete saved filter');
}
