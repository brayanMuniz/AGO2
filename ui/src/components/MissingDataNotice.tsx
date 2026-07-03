import React from 'react';
import { Link } from 'react-router-dom';
import DeleteImageButton from './DeleteImageButton';
import TopBar from './TopBar';

interface MissingDataNoticeProps {
  fileName: string;
  hash: string;
  imageId: number;
}

const MissingDataNotice: React.FC<MissingDataNoticeProps> = ({
  fileName,
  hash,
  imageId,
}) => {
  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans">
      <TopBar />

      <div className="flex-1 flex items-center justify-center p-8">
        <div className="max-w-lg w-full bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-8 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-full bg-[#2a2a35] text-[#fca5a5]">
            {/* Warning / Missing Data Icon */}
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
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>

          <h1 className="text-xl font-bold text-white mb-2">Missing Metadata</h1>
          <p className="text-gray-400 mb-6">
            This file exists in your gallery, but it does not currently have any associated metadata or tags.
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

export default MissingDataNotice;
