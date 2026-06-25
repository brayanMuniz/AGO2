import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { deleteImage } from '../api/images';

interface DeleteImageButtonProps {
  imageId: number;
  redirectTo?: string;
  onDeleted?: () => void;
  className?: string;
  variant?: 'icon' | 'button';
}

const DeleteImageButton: React.FC<DeleteImageButtonProps> = ({
  imageId,
  redirectTo = '/',
  onDeleted,
  className = '',
  variant = 'button',
}) => {
  const navigate = useNavigate();
  const [confirming, setConfirming] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleDelete = async () => {
    setDeleting(true);
    setError(null);

    try {
      await deleteImage(imageId);
      if (onDeleted) {
        onDeleted();
        setConfirming(false);
      } else {
        navigate(redirectTo);
      }
    } catch (err: any) {
      setError(err.message || 'Failed to delete image.');
      setDeleting(false);
      setConfirming(false);
    }
  };

  if (confirming) {
    return (
      <div
        className={`flex items-center gap-2 ${className}`}
        onClick={(event) => event.stopPropagation()}
      >
        <span className="text-xs text-red-300">Delete?</span>
        <button
          type="button"
          onClick={handleDelete}
          disabled={deleting}
          className="px-2 py-0.5 text-xs rounded bg-red-600 hover:bg-red-500 text-white disabled:opacity-50"
        >
          {deleting ? '...' : 'Yes'}
        </button>
        <button
          type="button"
          onClick={() => setConfirming(false)}
          disabled={deleting}
          className="px-2 py-0.5 text-xs rounded bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300"
        >
          No
        </button>
        {error && <span className="text-xs text-red-400">{error}</span>}
      </div>
    );
  }

  if (variant === 'icon') {
    return (
      <button
        type="button"
        onClick={(event) => {
          event.preventDefault();
          event.stopPropagation();
          setConfirming(true);
        }}
        aria-label="Delete image"
        className={`rounded-full p-1 text-gray-400 hover:text-red-400 transition-colors ${className}`}
      >
        <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
      </button>
    );
  }

  return (
    <button
      type="button"
      onClick={() => setConfirming(true)}
      className={`px-3 py-1.5 text-sm rounded bg-[#2a2a35] hover:bg-red-900/40 hover:text-red-300 text-gray-300 transition-colors ${className}`}
    >
      Delete
    </button>
  );
};

export default DeleteImageButton;
