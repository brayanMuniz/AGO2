import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { deleteImage } from '../api/images';

interface DeleteImageButtonProps {
  imageId: number;
  redirectTo?: string;
  onDeleted?: () => boolean | void;
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
  const [deleted, setDeleted] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleDelete = async () => {
    setDeleting(true);
    setError(null);

    try {
      await deleteImage(imageId);
      setDeleting(false);
      setConfirming(false);
      setDeleted(true);

      window.setTimeout(() => {
        if (onDeleted) {
          const handled = onDeleted();
          if (!handled) {
            navigate(redirectTo);
          }
        } else {
          navigate(redirectTo);
        }
      }, 900);
    } catch (err: any) {
      setError(err.message || 'Failed to delete image.');
      setDeleting(false);
      setConfirming(false);
    }
  };

  if (deleted) {
    return (
      <div className={`flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium rounded bg-green-500/20 text-green-400 border border-green-500/30 shadow-lg ${className}`}>
        <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
          <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
        </svg>
        <span>Deleted!</span>
      </div>
    );
  }

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
          className="px-2 py-0.5 text-xs rounded bg-red-600 hover:bg-red-500 text-white disabled:opacity-50 cursor-pointer"
        >
          {deleting ? '...' : 'Yes'}
        </button>
        <button
          type="button"
          onClick={() => setConfirming(false)}
          disabled={deleting}
          className="px-2 py-0.5 text-xs rounded bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 cursor-pointer"
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
        className={`rounded-full p-1 text-gray-400 hover:text-red-400 transition-colors cursor-pointer ${className}`}
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
      className={`px-3 py-1.5 text-sm rounded bg-[#2a2a35] hover:bg-red-900/40 hover:text-red-300 text-gray-300 transition-colors cursor-pointer ${className}`}
    >
      Delete
    </button>
  );
};

export default DeleteImageButton;
