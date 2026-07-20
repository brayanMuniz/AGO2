import React from 'react';

interface FavoriteStarProps {
  isFavorite: boolean;
  onToggle: () => void;
  className?: string;
  size?: 'sm' | 'md';
}

const FavoriteStar: React.FC<FavoriteStarProps> = ({
  isFavorite,
  onToggle,
  className = '',
  size = 'md',
}) => {
  const dimension = size === 'sm' ? 'h-4 w-4' : 'h-5 w-5';

  return (
    <button
      type="button"
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onToggle();
      }}
      aria-label={isFavorite ? 'Remove from favorites' : 'Add to favorites'}
      className={`rounded-full p-1 transition-colors cursor-pointer ${
        isFavorite
          ? 'text-yellow-400 hover:text-yellow-300'
          : 'text-gray-400 hover:text-yellow-400'
      } ${className}`}
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill={isFavorite ? 'currentColor' : 'none'}
        stroke="currentColor"
        strokeWidth={1.5}
        className={dimension}
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          d="M11.48 3.499a.562.562 0 011.04 0l2.125 5.111a.563.563 0 00.475.345l5.518.442c.499.04.701.663.321.988l-4.204 3.602a.563.563 0 00-.182.557l1.285 5.385a.562.562 0 01-.84.61l-4.725-2.885a.563.563 0 00-.586 0L6.982 20.54a.562.562 0 01-.84-.61l1.285-5.386a.562.562 0 00-.182-.557l-4.204-3.602a.563.563 0 01.321-.988l5.518-.442a.563.563 0 00.475-.345L11.48 3.5z"
        />
      </svg>
    </button>
  );
};

export default FavoriteStar;
