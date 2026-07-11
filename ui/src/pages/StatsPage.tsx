import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import TopBar from '../components/TopBar';

// --- Types ---

interface LibraryStats {
  total_images: number;
  total_duplicates: number;
  total_favorites: number;
  total_disk_space: number;
  unorganized_queue: number;
}

interface TagLeaderboardEntry {
  name: string;
  category: string;
  count: number;
}

interface RatingDistribution {
  rating: string;
  count: number;
}

interface PredictiveTagEntry {
  name: string;
  total_count: number;
  general_pct: number;
  sensitive_pct: number;
  questionable_pct: number;
  explicit_pct: number;
}

interface ArtistProfile {
  name: string;
  total_count: number;
  favorite_count: number;
  rating_breakdown: Record<string, number>;
  top_tags: TagLeaderboardEntry[];
}

interface StatsPayload {
  library: LibraryStats;
  tag_leaderboards: Record<string, TagLeaderboardEntry[]>;
  tag_leaderboards_favorites: Record<string, TagLeaderboardEntry[]>;
  rating_distribution: RatingDistribution[];
  predictive_by_rating: Record<string, PredictiveTagEntry[]>;
  artist_profiles: ArtistProfile[];
}

// --- Helpers ---

function formatBytes(bytes: number): string {
  if (bytes >= 1_000_000_000) return `${(bytes / 1_000_000_000).toFixed(2)} GB`;
  if (bytes >= 1_000_000) return `${(bytes / 1_000_000).toFixed(1)} MB`;
  if (bytes >= 1_000) return `${(bytes / 1_000).toFixed(0)} KB`;
  return `${bytes} B`;
}

const RATING_LABELS: Record<string, string> = {
  g: 'General',
  s: 'Sensitive',
  q: 'Questionable',
  e: 'Explicit',
};

const RATING_COLORS: Record<string, string> = {
  g: '#4ade80',
  s: '#facc15',
  q: '#fb923c',
  e: '#f87171',
};

const CATEGORY_DISPLAY: Record<string, { title: string; titleFav: string; color: string; icon: string }> = {
  artist: { title: 'Top Artists', titleFav: 'Most Favorited Artists', color: '#fca5a5', icon: '🎨' },
  character: { title: 'Top Characters', titleFav: 'Most Favorited Characters', color: '#4ade80', icon: '👤' },
  copyright: { title: 'Top Series', titleFav: 'Most Favorited Series', color: '#c084fc', icon: '📺' },
  general: { title: 'Top Tags', titleFav: 'Most Favorited Tags', color: '#60a5fa', icon: '🏷️' },
};

const DEFAULT_VISIBLE = 10;

// --- Sub-Components ---

const KPICard: React.FC<{
  label: string;
  value: string | number;
  icon: React.ReactNode;
  accent?: string;
  subtitle?: string;
}> = ({ label, value, icon, accent = '#60a5fa', subtitle }) => (
  <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-5 flex flex-col gap-1 hover:border-[#3a3a45] transition-colors">
    <div className="flex items-center justify-between mb-2">
      <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider">{label}</span>
      <span className="text-lg" style={{ color: accent }}>{icon}</span>
    </div>
    <span className="text-2xl font-bold text-gray-100 tracking-tight">{value}</span>
    {subtitle && <span className="text-xs text-gray-500 mt-0.5">{subtitle}</span>}
  </div>
);

const RatingBar: React.FC<{ distribution: RatingDistribution[] }> = ({ distribution }) => {
  const total = distribution.reduce((sum, d) => sum + d.count, 0);
  if (total === 0) return <div className="text-sm text-gray-500">No rating data available.</div>;

  return (
    <div>
      <div className="flex h-8 rounded-lg overflow-hidden border border-[#2a2a35]">
        {distribution.map((d) => {
          const pct = (d.count / total) * 100;
          if (pct < 0.5) return null;
          return (
            <div
              key={d.rating}
              className="flex items-center justify-center text-xs font-bold text-black/70 transition-all"
              style={{
                width: `${pct}%`,
                backgroundColor: RATING_COLORS[d.rating] || '#6b7280',
                minWidth: pct > 3 ? undefined : '20px',
              }}
              title={`${RATING_LABELS[d.rating] || d.rating}: ${d.count} (${pct.toFixed(1)}%)`}
            >
              {pct > 5 && (RATING_LABELS[d.rating]?.[0] || d.rating.toUpperCase())}
            </div>
          );
        })}
      </div>

      <div className="flex flex-wrap gap-4 mt-3">
        {distribution.map((d) => {
          const pct = ((d.count / total) * 100).toFixed(1);
          return (
            <div key={d.rating} className="flex items-center gap-2 text-xs text-gray-400">
              <span
                className="w-2.5 h-2.5 rounded-full shrink-0"
                style={{ backgroundColor: RATING_COLORS[d.rating] || '#6b7280' }}
              />
              <span className="font-medium text-gray-300">{RATING_LABELS[d.rating] || d.rating}</span>
              <span className="text-gray-500">
                {d.count.toLocaleString()} ({pct}%)
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
};

const SortToggle: React.FC<{
  mode: 'count' | 'favorites';
  onChange: (mode: 'count' | 'favorites') => void;
}> = ({ mode, onChange }) => (
  <div className="flex bg-[#15151a] rounded-md border border-[#2a2a35] overflow-hidden">
    <button
      type="button"
      onClick={() => onChange('count')}
      className={`px-2 py-0.5 text-[10px] font-medium transition-colors ${
        mode === 'count'
          ? 'bg-[#2a2a35] text-gray-200'
          : 'text-gray-500 hover:text-gray-300'
      }`}
    >
      All
    </button>
    <button
      type="button"
      onClick={() => onChange('favorites')}
      className={`px-2 py-0.5 text-[10px] font-medium transition-colors ${
        mode === 'favorites'
          ? 'bg-[#2a2a35] text-pink-300'
          : 'text-gray-500 hover:text-gray-300'
      }`}
    >
      ♥ Fav
    </button>
  </div>
);

const TagLeaderboardColumn: React.FC<{
  title: string;
  titleFav: string;
  entries: TagLeaderboardEntry[];
  entriesFav: TagLeaderboardEntry[];
  color: string;
  icon: string;
}> = ({ title, titleFav, entries, entriesFav, color, icon }) => {
  const [expanded, setExpanded] = useState(false);
  const [sortMode, setSortMode] = useState<'count' | 'favorites'>('count');

  const activeEntries = sortMode === 'favorites' ? entriesFav : entries;
  const activeTitle = sortMode === 'favorites' ? titleFav : title;
  const visible = expanded ? activeEntries : activeEntries.slice(0, DEFAULT_VISIBLE);

  return (
    <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-5 flex flex-col">
      <div className="flex items-center gap-2 mb-4">
        <span className="text-base">{icon}</span>
        <h3 className="font-bold text-sm text-gray-200 flex-1 truncate">{activeTitle}</h3>
        <SortToggle mode={sortMode} onChange={setSortMode} />
      </div>

      {activeEntries.length === 0 ? (
        <p className="text-sm text-gray-500">{sortMode === 'favorites' ? 'No favorites yet.' : 'No data yet.'}</p>
      ) : (
        <>
          <ul className="space-y-1 flex-1">
            {visible.map((entry, i) => {
              const query = sortMode === 'favorites' ? `${entry.name} favorite:true` : entry.name;
              return (
                <li key={entry.name}>
                  <Link
                    to={`/?tags=${encodeURIComponent(query)}`}
                    className="flex items-center gap-2 py-1.5 px-2 rounded hover:bg-[#2a2a35] transition-colors group"
                    title={`Search images for ${entry.name}${sortMode === 'favorites' ? ' (Favorites only)' : ''}`}
                  >
                    <span className="text-xs text-gray-600 w-5 text-right shrink-0 font-mono">
                      {i + 1}
                    </span>
                    <span
                      className="text-sm font-medium truncate flex-1 group-hover:underline flex items-center gap-1"
                      style={{ color }}
                    >
                      {entry.name}
                    </span>
                    <span className="text-xs text-gray-400 group-hover:text-gray-200 shrink-0 bg-[#2a2a35] group-hover:bg-[#3a3a45] px-2 py-0.5 rounded-full transition-colors font-mono">
                      {sortMode === 'favorites' ? `♥ ${entry.count}` : entry.count.toLocaleString()}
                    </span>
                  </Link>
                </li>
              );
            })}
          </ul>
          {activeEntries.length > DEFAULT_VISIBLE && (
            <button
              type="button"
              onClick={() => setExpanded(!expanded)}
              className="mt-3 text-xs text-[#60a5fa] hover:text-[#93c5fd] transition-colors self-start"
            >
              {expanded ? '← Show less' : `Show all ${activeEntries.length} →`}
            </button>
          )}
        </>
      )}
    </div>
  );
};

const PredictiveRatingColumn: React.FC<{
  title: string;
  ratingKey: 'g' | 's' | 'q' | 'e';
  entries: PredictiveTagEntry[];
  icon: string;
  headerClass: string;
  badgeClass: string;
  selectedTag?: PredictiveTagEntry | null;
  onSelectTag?: (entry: PredictiveTagEntry) => void;
}> = ({ title, ratingKey, entries, icon, headerClass, badgeClass, selectedTag, onSelectTag }) => {
  const [expanded, setExpanded] = useState(false);
  const visible = expanded ? entries : entries.slice(0, DEFAULT_VISIBLE);

  const getPercentage = (entry: PredictiveTagEntry) => {
    switch (ratingKey) {
      case 'g': return entry.general_pct;
      case 's': return entry.sensitive_pct;
      case 'q': return entry.questionable_pct;
      case 'e': return entry.explicit_pct;
    }
  };

  return (
    <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-5 flex flex-col">
      <div className="flex items-center gap-2 mb-4">
        <span className="text-base">{icon}</span>
        <h3 className={`font-bold text-sm ${headerClass} flex-1 truncate`}>{title}</h3>
      </div>

      {entries.length === 0 ? (
        <p className="text-sm text-gray-500">Not enough data (min 5 occurrences required).</p>
      ) : (
        <>
          <ul className="space-y-1 flex-1">
            {visible.map((entry, i) => {
              const pct = getPercentage(entry);
              const isSelected = selectedTag?.name === entry.name;
              return (
                <li
                  key={entry.name}
                  onClick={() => onSelectTag && onSelectTag(entry)}
                  className={`flex items-center gap-2 py-1.5 px-2 rounded cursor-pointer transition-colors ${
                    isSelected
                      ? 'bg-[#2a2a35] border border-[#60a5fa]/50'
                      : 'hover:bg-[#2a2a35]/50'
                  }`}
                >
                  <span className="text-xs text-gray-600 w-5 text-right shrink-0 font-mono">
                    {i + 1}
                  </span>
                  <span className="text-sm text-gray-200 truncate flex-1" title={entry.name}>
                    {entry.name}
                  </span>
                  <span
                    className={`text-xs font-semibold px-2 py-0.5 rounded-full border shrink-0 ${badgeClass}`}
                  >
                    {pct.toFixed(1)}%
                  </span>
                  <span className="text-[10px] text-gray-600 shrink-0 w-8 text-right" title="Total occurrences">
                    ×{entry.total_count}
                  </span>
                </li>
              );
            })}
          </ul>
          {entries.length > DEFAULT_VISIBLE && (
            <button
              type="button"
              onClick={() => setExpanded(!expanded)}
              className="mt-3 text-xs text-[#60a5fa] hover:text-[#93c5fd] transition-colors self-start"
            >
              {expanded ? '← Show less' : `Show all ${entries.length} →`}
            </button>
          )}
        </>
      )}
    </div>
  );
};

const MiniRatingBar: React.FC<{ breakdown: Record<string, number> }> = ({ breakdown }) => {
  const total = Object.values(breakdown).reduce((s, c) => s + c, 0);
  if (total === 0) return null;

  const ordered = ['g', 's', 'q', 'e'];

  return (
    <div className="flex h-2 rounded-full overflow-hidden bg-[#2a2a35] w-full">
      {ordered.map((r) => {
        const count = breakdown[r] || 0;
        const pct = (count / total) * 100;
        if (pct < 0.5) return null;
        return (
          <div
            key={r}
            style={{ width: `${pct}%`, backgroundColor: RATING_COLORS[r] }}
            title={`${RATING_LABELS[r]}: ${count} (${pct.toFixed(0)}%)`}
          />
        );
      })}
    </div>
  );
};

const ArtistCard: React.FC<{ profile: ArtistProfile }> = ({ profile }) => {
  const favPct = profile.total_count > 0
    ? ((profile.favorite_count / profile.total_count) * 100).toFixed(0)
    : '0';

  return (
    <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-4 hover:border-[#3a3a45] transition-colors flex flex-col gap-3">
      {/* Header */}
      <div className="flex items-center justify-between">
        <Link
          to={`/?tags=${encodeURIComponent(profile.name)}`}
          className="text-sm font-bold text-[#fca5a5] hover:text-pink-300 hover:underline truncate inline-flex items-center gap-1"
          title={`Click to search images by artist #${profile.name}`}
        >
          {profile.name} ↗
        </Link>
        <div className="flex items-center gap-2 shrink-0 text-xs text-gray-500">
          <span>{profile.total_count} img</span>
          <Link
            to={`/?tags=${encodeURIComponent(`${profile.name} favorite:true`)}`}
            className="text-pink-400 hover:text-pink-300 hover:underline inline-flex items-center gap-0.5"
            title={`${profile.favorite_count} favorited (${favPct}%) - Click to search artist favorites`}
          >
            ♥ {profile.favorite_count}
          </Link>
        </div>
      </div>

      {/* Rating bar */}
      <MiniRatingBar breakdown={profile.rating_breakdown} />

      {/* Rating breakdown numbers */}
      <div className="flex gap-2 flex-wrap">
        {['g', 's', 'q', 'e'].map((r) => {
          const count = profile.rating_breakdown[r] || 0;
          if (count === 0) return null;
          return (
            <Link
              key={r}
              to={`/?tags=${encodeURIComponent(`${profile.name} rating:${r}`)}`}
              className="inline-flex items-center gap-1 text-[10px] text-gray-400 hover:text-gray-200 hover:underline transition-colors"
              title={`Search images by ${profile.name} with rating ${RATING_LABELS[r]}`}
            >
              <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{ backgroundColor: RATING_COLORS[r] }} />
              {RATING_LABELS[r]?.[0]}: {count}
            </Link>
          );
        })}
      </div>

      {/* Top tags */}
      {profile.top_tags.length > 0 && (
        <div className="flex flex-wrap gap-1.5 mt-1">
          {profile.top_tags.map((tag) => (
            <Link
              key={tag.name}
              to={`/?tags=${encodeURIComponent(`${profile.name} ${tag.name}`)}`}
              className="text-[10px] px-2 py-0.5 rounded-full bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 hover:text-white border border-[#3a3a45] hover:border-[#fca5a5]/50 transition-colors inline-flex items-center gap-1"
              title={`Click to search images by ${profile.name} with tag #${tag.name}`}
            >
              {tag.name}
              <span className="text-gray-500 font-mono">{tag.count}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
};

// --- Main Page ---

const StatsPage: React.FC = () => {
  const [stats, setStats] = useState<StatsPayload | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedPredictiveTag, setSelectedPredictiveTag] = useState<PredictiveTagEntry | null>(null);

  useEffect(() => {
    const fetchStats = async () => {
      setLoading(true);
      setError(null);
      try {
        const res = await fetch('/api/stats');
        if (!res.ok) throw new Error('Failed to fetch stats');
        const data: StatsPayload = await res.json();
        setStats(data);
      } catch (err: any) {
        setError(err.message || 'Unknown error');
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
  }, []);

  const favRatio = stats && stats.library.total_images > 0
    ? ((stats.library.total_favorites / stats.library.total_images) * 100).toFixed(1)
    : '0';

  return (
    <div className="min-h-screen bg-[#0e0e12] flex flex-col text-gray-300 font-sans">
      <TopBar />

      <main className="flex-1 overflow-y-auto hide-scrollbar">
        <div className="max-w-6xl mx-auto px-6 py-8">
          {/* Page Header */}
          <div className="mb-8">
            <h1 className="text-2xl font-bold text-gray-100 tracking-tight">Library Statistics</h1>
            <p className="text-sm text-gray-500 mt-1">An overview of your gallery's composition and insights.</p>
          </div>

          {loading ? (
            <div className="flex items-center justify-center h-64 text-gray-400">
              <div className="flex flex-col items-center gap-3">
                <div className="w-8 h-8 border-2 border-[#60a5fa] border-t-transparent rounded-full animate-spin" />
                <span className="text-sm">Loading statistics...</span>
              </div>
            </div>
          ) : error ? (
            <div className="flex items-center justify-center h-64 text-red-400">
              <span>{error}</span>
            </div>
          ) : stats ? (
            <div className="flex flex-col gap-8">
              {/* --- KPI Scorecard --- */}
              <section>
                <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">Overview</h2>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                  <KPICard
                    label="Total Images"
                    value={stats.library.total_images.toLocaleString()}
                    icon={
                      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5">
                        <path fillRule="evenodd" d="M1 5.25A2.25 2.25 0 0 1 3.25 3h13.5A2.25 2.25 0 0 1 19 5.25v9.5A2.25 2.25 0 0 1 16.75 17H3.25A2.25 2.25 0 0 1 1 14.75v-9.5Zm1.5 5.81v3.69c0 .414.336.75.75.75h13.5a.75.75 0 0 0 .75-.75v-2.69l-2.22-2.219a.75.75 0 0 0-1.06 0l-1.91 1.909-4.72-4.719a.75.75 0 0 0-1.06 0L2.5 11.06Z" clipRule="evenodd" />
                      </svg>
                    }
                    accent="#60a5fa"
                    subtitle={stats.library.unorganized_queue > 0 ? `${stats.library.unorganized_queue} unmatched` : undefined}
                  />
                  <KPICard
                    label="Favorites"
                    value={stats.library.total_favorites.toLocaleString()}
                    icon={
                      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5">
                        <path d="M9.653 16.915l-.005-.003-.019-.01a20.759 20.759 0 0 1-1.162-.682 22.045 22.045 0 0 1-2.582-1.9C4.045 12.733 2 10.352 2 7.5a4.5 4.5 0 0 1 8-2.828A4.5 4.5 0 0 1 18 7.5c0 2.852-2.044 5.233-3.885 6.82a22.049 22.049 0 0 1-3.744 2.582l-.019.01-.005.003h-.002a.723.723 0 0 1-.692 0h-.002Z" />
                      </svg>
                    }
                    accent="#f472b6"
                    subtitle={`${favRatio}% of library`}
                  />
                  <KPICard
                    label="Library Size"
                    value={formatBytes(stats.library.total_disk_space)}
                    icon={
                      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5">
                        <path d="M10.362 1.093a.75.75 0 0 0-.724 0L2.523 5.018 10 9.143l7.477-4.125-7.115-3.925ZM18 6.443l-7.25 4v8.25l6.862-3.786A.75.75 0 0 0 18 14.25V6.443ZM9.25 18.693v-8.25l-7.25-4v7.807a.75.75 0 0 0 .388.657l6.862 3.786Z" />
                      </svg>
                    }
                    accent="#a78bfa"
                  />
                </div>
              </section>

              {/* --- Rating Distribution --- */}
              <section>
                <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">
                  Rating Distribution
                </h2>
                <div className="bg-[#1c1c24] border border-[#2a2a35] rounded-xl p-5">
                  <RatingBar distribution={stats.rating_distribution} />
                </div>
              </section>

              {/* --- Leaderboards --- */}
              <section>
                <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">
                  Leaderboards
                </h2>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                  {(['artist', 'character', 'copyright', 'general'] as const).map((cat) => {
                    const cfg = CATEGORY_DISPLAY[cat];
                    return (
                      <TagLeaderboardColumn
                        key={cat}
                        title={cfg.title}
                        titleFav={cfg.titleFav}
                        entries={stats.tag_leaderboards[cat] || []}
                        entriesFav={stats.tag_leaderboards_favorites[cat] || []}
                        color={cfg.color}
                        icon={cfg.icon}
                      />
                    );
                  })}
                </div>
              </section>

              {/* --- Artist Insights --- */}
              {stats.artist_profiles && stats.artist_profiles.length > 0 && (
                <section>
                  <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">
                    Artist Insights
                  </h2>
                  <p className="text-xs text-gray-500 mb-4">
                    Rating breakdown, favorite ratio, and top descriptive tags for your most represented artists.
                  </p>
                  <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                    {stats.artist_profiles.map((profile) => (
                      <ArtistCard key={profile.name} profile={profile} />
                    ))}
                  </div>
                </section>
              )}

              {/* --- Predictive Analytics --- */}
              <section>
                <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-3">
                  Predictive Tag Analytics
                </h2>
                <p className="text-xs text-gray-500 mb-4">
                  Analyzes which descriptive (general) tags correlate most strongly with General, Sensitive, Questionable, and Explicit ratings. Click any tag row to see its full rating distribution below.
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                  <PredictiveRatingColumn
                    title="General-Leaning"
                    ratingKey="g"
                    entries={stats.predictive_by_rating?.['g'] || []}
                    icon="🛡️"
                    headerClass="text-emerald-400"
                    badgeClass="bg-emerald-500/15 text-emerald-400 border-emerald-500/30"
                    selectedTag={selectedPredictiveTag}
                    onSelectTag={(entry) => setSelectedPredictiveTag(selectedPredictiveTag?.name === entry.name ? null : entry)}
                  />
                  <PredictiveRatingColumn
                    title="Sensitive-Leaning"
                    ratingKey="s"
                    entries={stats.predictive_by_rating?.['s'] || []}
                    icon="⚠️"
                    headerClass="text-yellow-400"
                    badgeClass="bg-yellow-500/15 text-yellow-400 border-yellow-500/30"
                    selectedTag={selectedPredictiveTag}
                    onSelectTag={(entry) => setSelectedPredictiveTag(selectedPredictiveTag?.name === entry.name ? null : entry)}
                  />
                  <PredictiveRatingColumn
                    title="Questionable-Leaning"
                    ratingKey="q"
                    entries={stats.predictive_by_rating?.['q'] || []}
                    icon="❓"
                    headerClass="text-orange-400"
                    badgeClass="bg-orange-500/15 text-orange-400 border-orange-500/30"
                    selectedTag={selectedPredictiveTag}
                    onSelectTag={(entry) => setSelectedPredictiveTag(selectedPredictiveTag?.name === entry.name ? null : entry)}
                  />
                  <PredictiveRatingColumn
                    title="Explicit-Leaning"
                    ratingKey="e"
                    entries={stats.predictive_by_rating?.['e'] || []}
                    icon="🔥"
                    headerClass="text-red-400"
                    badgeClass="bg-red-500/15 text-red-400 border-red-500/30"
                    selectedTag={selectedPredictiveTag}
                    onSelectTag={(entry) => setSelectedPredictiveTag(selectedPredictiveTag?.name === entry.name ? null : entry)}
                  />
                </div>

                {/* Selected Tag Full Distribution Popup */}
                {selectedPredictiveTag && (
                  <div className="mt-6 bg-[#1c1c24] border-2 border-[#60a5fa]/50 rounded-2xl p-6 shadow-2xl transition-all">
                    <div className="flex items-center justify-between mb-4">
                      <div className="flex items-center gap-3">
                        <span className="text-lg font-bold text-gray-100 flex items-center gap-1.5">
                          Rating Distribution for tag{' '}
                          <Link
                            to={`/?tags=${encodeURIComponent(selectedPredictiveTag.name)}`}
                            className="text-[#60a5fa] hover:text-[#93c5fd] hover:underline inline-flex items-center gap-1 transition-colors"
                            title="Click to search all images with this tag"
                          >
                            #{selectedPredictiveTag.name} ↗
                          </Link>
                        </span>
                        <Link
                          to={`/?tags=${encodeURIComponent(selectedPredictiveTag.name)}`}
                          className="text-xs bg-[#2a2a35] hover:bg-[#3a3a45] text-gray-300 px-2.5 py-1 rounded-full font-mono transition-colors"
                          title="Click to search all images with this tag"
                        >
                          ×{selectedPredictiveTag.total_count} occurrences ↗
                        </Link>
                      </div>
                      <button
                        type="button"
                        onClick={() => setSelectedPredictiveTag(null)}
                        className="text-gray-400 hover:text-gray-100 text-sm px-3 py-1 rounded-lg bg-[#2a2a35] hover:bg-[#3a3a45] transition-colors"
                      >
                        Close ✕
                      </button>
                    </div>

                    {/* Visual colored bar */}
                    <div className="flex h-4 rounded-full overflow-hidden bg-[#2a2a35] mb-5 border border-[#3a3a45]">
                      {selectedPredictiveTag.general_pct > 0 && (
                        <div
                          style={{ width: `${selectedPredictiveTag.general_pct}%`, backgroundColor: RATING_COLORS.g }}
                          title={`General: ${selectedPredictiveTag.general_pct.toFixed(1)}%`}
                        />
                      )}
                      {selectedPredictiveTag.sensitive_pct > 0 && (
                        <div
                          style={{ width: `${selectedPredictiveTag.sensitive_pct}%`, backgroundColor: RATING_COLORS.s }}
                          title={`Sensitive: ${selectedPredictiveTag.sensitive_pct.toFixed(1)}%`}
                        />
                      )}
                      {selectedPredictiveTag.questionable_pct > 0 && (
                        <div
                          style={{ width: `${selectedPredictiveTag.questionable_pct}%`, backgroundColor: RATING_COLORS.q }}
                          title={`Questionable: ${selectedPredictiveTag.questionable_pct.toFixed(1)}%`}
                        />
                      )}
                      {selectedPredictiveTag.explicit_pct > 0 && (
                        <div
                          style={{ width: `${selectedPredictiveTag.explicit_pct}%`, backgroundColor: RATING_COLORS.e }}
                          title={`Explicit: ${selectedPredictiveTag.explicit_pct.toFixed(1)}%`}
                        />
                      )}
                    </div>

                    {/* Exact Percentage Cards - Clickable links to SearchPage */}
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                      <Link
                        to={`/?tags=${encodeURIComponent(`${selectedPredictiveTag.name} rating:g`)}`}
                        className="flex items-center justify-between bg-[#15151a] border border-emerald-500/30 rounded-xl p-3.5 hover:bg-[#20202a] hover:border-emerald-500/60 transition-all cursor-pointer group"
                        title="Search images with this tag and rating: General"
                      >
                        <div className="flex items-center gap-2">
                          <span className="w-3 h-3 rounded-full shrink-0" style={{ backgroundColor: RATING_COLORS.g }} />
                          <span className="text-xs font-semibold text-gray-300 group-hover:text-white transition-colors">
                            General ↗
                          </span>
                        </div>
                        <span className="text-base font-bold text-emerald-400 font-mono">
                          {selectedPredictiveTag.general_pct.toFixed(1)}%
                        </span>
                      </Link>

                      <Link
                        to={`/?tags=${encodeURIComponent(`${selectedPredictiveTag.name} rating:s`)}`}
                        className="flex items-center justify-between bg-[#15151a] border border-yellow-500/30 rounded-xl p-3.5 hover:bg-[#20202a] hover:border-yellow-500/60 transition-all cursor-pointer group"
                        title="Search images with this tag and rating: Sensitive"
                      >
                        <div className="flex items-center gap-2">
                          <span className="w-3 h-3 rounded-full shrink-0" style={{ backgroundColor: RATING_COLORS.s }} />
                          <span className="text-xs font-semibold text-gray-300 group-hover:text-white transition-colors">
                            Sensitive ↗
                          </span>
                        </div>
                        <span className="text-base font-bold text-yellow-400 font-mono">
                          {selectedPredictiveTag.sensitive_pct.toFixed(1)}%
                        </span>
                      </Link>

                      <Link
                        to={`/?tags=${encodeURIComponent(`${selectedPredictiveTag.name} rating:q`)}`}
                        className="flex items-center justify-between bg-[#15151a] border border-orange-500/30 rounded-xl p-3.5 hover:bg-[#20202a] hover:border-orange-500/60 transition-all cursor-pointer group"
                        title="Search images with this tag and rating: Questionable"
                      >
                        <div className="flex items-center gap-2">
                          <span className="w-3 h-3 rounded-full shrink-0" style={{ backgroundColor: RATING_COLORS.q }} />
                          <span className="text-xs font-semibold text-gray-300 group-hover:text-white transition-colors">
                            Questionable ↗
                          </span>
                        </div>
                        <span className="text-base font-bold text-orange-400 font-mono">
                          {selectedPredictiveTag.questionable_pct.toFixed(1)}%
                        </span>
                      </Link>

                      <Link
                        to={`/?tags=${encodeURIComponent(`${selectedPredictiveTag.name} rating:e`)}`}
                        className="flex items-center justify-between bg-[#15151a] border border-red-500/30 rounded-xl p-3.5 hover:bg-[#20202a] hover:border-red-500/60 transition-all cursor-pointer group"
                        title="Search images with this tag and rating: Explicit"
                      >
                        <div className="flex items-center gap-2">
                          <span className="w-3 h-3 rounded-full shrink-0" style={{ backgroundColor: RATING_COLORS.e }} />
                          <span className="text-xs font-semibold text-gray-300 group-hover:text-white transition-colors">
                            Explicit ↗
                          </span>
                        </div>
                        <span className="text-base font-bold text-red-400 font-mono">
                          {selectedPredictiveTag.explicit_pct.toFixed(1)}%
                        </span>
                      </Link>
                    </div>
                  </div>
                )}
              </section>
            </div>
          ) : null}
        </div>
      </main>
    </div>
  );
};

export default StatsPage;
