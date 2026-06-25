export async function updateFavorite(id: number, isFavorite: boolean): Promise<void> {
  const response = await fetch(`/api/image/${id}/favorite`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ is_favorite: isFavorite }),
  });

  if (!response.ok) {
    throw new Error('Failed to update favorite status.');
  }
}

export async function deleteImage(id: number): Promise<void> {
  const response = await fetch(`/api/image/${id}`, {
    method: 'DELETE',
  });

  if (!response.ok) {
    throw new Error('Failed to delete image.');
  }
}

export async function exportAlbum(albumName: string, imageIds: number[]): Promise<void> {
  const response = await fetch('/api/album/export', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ album_name: albumName, image_ids: imageIds }),
  });

  if (!response.ok) {
    throw new Error('Failed to export album.');
  }
}
