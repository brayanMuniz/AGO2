import React from 'react';
import { Link, useLocation } from 'react-router-dom';

const TopBar: React.FC = () => {
  const location = useLocation();

  return (
    <header className="h-12 shrink-0 border-b border-[#2a2a35] bg-[#1c1c24] flex items-center justify-between px-4">
      {/* Left Side: Logo */}
      <div className="flex items-center gap-6">
        <Link
          to="/"
          className="text-lg font-bold tracking-wide text-[#60a5fa] hover:text-[#93c5fd] transition-colors"
        >
          AGO
        </Link>
      </div>

      {/* Right Side: Navigation */}
      <div className="flex items-center gap-4">
        <Link
          to="/stats"
          className={`text-sm transition-colors ${location.pathname === '/stats'
              ? 'text-white font-medium'
              : 'text-gray-400 hover:text-gray-200'
            }`}
        >
          Stats
        </Link>
        <Link
          to="/settings"
          className={`text-sm transition-colors ${location.pathname === '/settings' || location.pathname.startsWith('/settings/')
              ? 'text-white font-medium'
              : 'text-gray-400 hover:text-gray-200'
            }`}
        >
          Settings
        </Link>
      </div>
    </header>
  );
};

export default TopBar;
