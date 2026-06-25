import React from 'react';
import { Link } from 'react-router-dom';

const TopBar: React.FC = () => {
  return (
    <header className="h-12 shrink-0 border-b border-[#2a2a35] bg-[#1c1c24] flex items-center px-4">
      <Link
        to="/"
        className="text-lg font-bold tracking-wide text-[#60a5fa] hover:text-[#93c5fd] transition-colors"
      >
        AGO
      </Link>
    </header>
  );
};

export default TopBar;
