import React, { useState } from 'react';
import { exportAlbum } from '../api/images';

interface ExportAlbumModalProps {
  imageIds: number[];
  onClose: () => void;
}

const ExportAlbumModal: React.FC<ExportAlbumModalProps> = ({ imageIds, onClose }) => {
  const [albumName, setAlbumName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!albumName.trim()) return;

    setLoading(true);
    setError(null);

    try {
      await exportAlbum(albumName.trim(), imageIds);
      setSuccess(`Exported ${imageIds.length} image(s) to "${albumName.trim()}".`);
    } catch (err: any) {
      setError(err.message || 'Failed to export album.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
      <div className="w-full max-w-md rounded-xl border border-[#2a2a35] bg-[#1c1c24] p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-white">Export Album</h2>
          <button
            type="button"
            onClick={onClose}
            className="text-gray-400 hover:text-white"
          >
            ×
          </button>
        </div>

        <p className="text-sm text-gray-400 mb-4">
          Export {imageIds.length} selected image(s) into a new album folder.
        </p>

        <form onSubmit={handleSubmit} className="space-y-4">
          <input
            type="text"
            value={albumName}
            onChange={(event) => setAlbumName(event.target.value)}
            placeholder="Album name"
            className="w-full bg-[#2a2a35] text-white px-3 py-2 text-sm border border-transparent focus:border-blue-500 focus:outline-none rounded-sm"
            autoFocus
          />

          {error && <p className="text-sm text-red-400">{error}</p>}
          {success && <p className="text-sm text-green-400">{success}</p>}

          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-3 py-1.5 text-sm rounded bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300"
            >
              Close
            </button>
            <button
              type="submit"
              disabled={loading || !albumName.trim() || !!success}
              className="px-3 py-1.5 text-sm rounded bg-[#60a5fa] hover:bg-[#3b82f6] text-white disabled:opacity-50"
            >
              {loading ? 'Exporting...' : 'Export'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default ExportAlbumModal;
