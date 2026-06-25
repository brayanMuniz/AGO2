import React from 'react';
import { Link } from 'react-router-dom';
import DeleteImageButton from './DeleteImageButton';
import TopBar from './TopBar';

interface DuplicateImageNoticeProps {
  fileName: string;
  hash: string;
  imageId: number;
  originalImageId?: number;
}

const DuplicateImageNotice: React.FC<DuplicateImageNoticeProps> = ({
  fileName,
  hash,
  imageId,
  originalImageId,
}) => {
  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans">
      <TopBar />

      <div className="flex-1 flex items-center justify-center p-8">
        <div className="max-w-lg w-full bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-8 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-[#2a2a35] text-[#fb923c]">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              className="h-7 w-7"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
              />
            </svg>
          </div>

          <h1 className="text-xl font-bold text-white mb-2">Duplicate Image</h1>
          <p className="text-gray-400 mb-6">
            This file has already been added to your gallery. It matches an existing image and
            does not have its own metadata.
          </p>

          <dl className="text-left text-sm space-y-2 mb-6 bg-[#111115] rounded-lg p-4">
            <div className="flex justify-between gap-4">
              <dt className="text-gray-500 shrink-0">File</dt>
              <dd className="text-gray-300 truncate">{fileName}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-gray-500 shrink-0">Record ID</dt>
              <dd className="text-gray-300">{imageId}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-gray-500 shrink-0">Hash</dt>
              <dd className="text-gray-400 truncate font-mono text-xs">{hash}</dd>
            </div>
          </dl>

          <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
            {originalImageId ? (
              <Link
                to={`/image/${originalImageId}`}
                className="inline-block px-4 py-2 bg-[#60a5fa] hover:bg-[#3b82f6] text-white rounded-md text-sm font-medium transition-colors"
              >
                View original image
              </Link>
            ) : (
              <p className="text-xs text-gray-500 mb-2 sm:mb-0">
                Original image link unavailable until the API returns <code>has_duplicate</code>.
              </p>
            )}
            <Link
              to="/"
              className="inline-block px-4 py-2 bg-[#2a2a35] hover:bg-[#3a3a45] text-[#60a5fa] rounded-md text-sm font-medium transition-colors"
            >
              Back to home
            </Link>
            <DeleteImageButton imageId={imageId} redirectTo="/" />
          </div>
        </div>
      </div>
    </div>
  );
};

export default DuplicateImageNotice;
