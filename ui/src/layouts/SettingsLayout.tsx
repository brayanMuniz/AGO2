import React from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import TopBar from '../components/TopBar';

const SettingsLayout: React.FC = () => {
  const navItems = [
    { path: '/settings', label: 'General', exact: true },
    { path: '/settings/sync', label: 'Sync' },
    { path: '/settings/match', label: 'Match' },
    { path: '/settings/duplicates', label: 'Duplicates' },
  ];

  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans">
      <TopBar />

      {/* Main container stretches full width/height, exactly like SearchPage */}
      <div className="flex flex-1 overflow-hidden">

        {/* Left Sidebar: Exact same width and background as SearchPage */}
        <aside className="w-72 bg-[#1c1c24] border-r border-[#2a2a35] flex flex-col flex-shrink-0 p-4 gap-1.5">
          <h2 className="font-bold text-gray-200 mb-2 px-2 text-sm">Settings</h2>
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              end={item.exact}
              className={({ isActive }) =>
                `px-3 py-2 rounded text-sm transition-colors font-medium block ${isActive
                  ? 'bg-[#2a2a35] text-[#93c5fd]'
                  : 'text-gray-400 hover:text-gray-200 hover:bg-[#2a2a35]/60'
                }`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </aside>

        {/* Right Main Content Area */}
        <main className="flex-1 flex flex-col overflow-hidden bg-[#0e0e12]">
          <div className="flex-1 overflow-y-auto p-8 hide-scrollbar">
            <Outlet />
          </div>
        </main>

      </div>
    </div>
  );
};

export default SettingsLayout;
