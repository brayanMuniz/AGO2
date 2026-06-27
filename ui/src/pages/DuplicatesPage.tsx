import React from 'react';

const DuplicatesPage: React.FC = () => {
  return (
    <div className="h-full flex flex-col">
      <h1 className="text-2xl font-bold text-gray-200 mb-6">Manage Duplicates</h1>
      <div className="flex-1 bg-[#0e0e12] border border-[#2a2a35] rounded-lg p-6 flex items-center justify-center">
        <p className="text-gray-500">Detected duplicate images will be displayed here.</p>
      </div>
    </div>
  );
};

export default DuplicatesPage;
