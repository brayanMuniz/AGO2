import React from 'react';
import TopBar from '../components/TopBar';

const SettingsPage: React.FC = () => {
  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans">
      <TopBar />

      <main className="flex-1 p-8 overflow-y-auto hide-scrollbar">
        <div className="max-w-4xl mx-auto">
          <h1 className="text-2xl font-bold text-gray-200 mb-6">Settings</h1>

          <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-lg p-6">
            <p className="text-gray-400">Settings and application preferences will go here.</p>
          </div>
        </div>
      </main>
    </div>
  );
};

export default SettingsPage;
