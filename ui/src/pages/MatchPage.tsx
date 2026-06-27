import React from 'react';

const MatchPage: React.FC = () => {
  return (
    <div className="h-full flex flex-col">
      <h1 className="text-2xl font-bold text-gray-200 mb-6">Match Images</h1>
      <div className="flex-1 bg-[#0e0e12] border border-[#2a2a35] rounded-lg p-6 flex items-center justify-center">
        <p className="text-gray-500">Items that still need to be processed and matched.</p>
      </div>
    </div>
  );
};

export default MatchPage;
