import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { Search, RefreshCw, Monitor, Apple, Terminal, CheckCircle } from 'lucide-react';
import { getGames, searchGames, refreshCatalogue } from '../api/client';
import type { Game } from '../types';

export default function LibraryPage() {
  const [searchQuery, setSearchQuery] = useState('');
  const [page, setPage] = useState(0);
  const limit = 24;
  const queryClient = useQueryClient();

  const { data, isLoading, error } = useQuery({
    queryKey: ['games', searchQuery, page],
    queryFn: () =>
      searchQuery
        ? searchGames(searchQuery, limit, page * limit)
        : getGames(limit, page * limit),
  });

  const refreshMutation = useMutation({
    mutationFn: refreshCatalogue,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['games'] });
    },
  });

  const games = data?.data ?? [];
  const total = data?.total ?? 0;
  const totalPages = Math.ceil(total / limit);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Game Library</h1>
          <p className="text-gray-400">
            {total} game{total !== 1 ? 's' : ''} in your collection
          </p>
        </div>
        <button
          onClick={() => refreshMutation.mutate()}
          disabled={refreshMutation.isPending}
          className="flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-700 disabled:bg-purple-800 rounded-lg transition-colors"
        >
          <RefreshCw size={18} className={refreshMutation.isPending ? 'animate-spin' : ''} />
          {refreshMutation.isPending ? 'Refreshing...' : 'Refresh Catalogue'}
        </button>
      </div>

      {/* Search */}
      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" size={20} />
        <input
          type="text"
          placeholder="Search games..."
          value={searchQuery}
          onChange={(e) => {
            setSearchQuery(e.target.value);
            setPage(0);
          }}
          className="w-full pl-10 pr-4 py-2 bg-gray-800 border border-gray-700 rounded-lg focus:outline-none focus:border-purple-500 text-white placeholder-gray-400"
        />
      </div>

      {/* Status Messages */}
      {refreshMutation.isSuccess && (
        <div className="p-4 bg-green-900/50 border border-green-700 rounded-lg text-green-300">
          Catalogue refreshed! Found {refreshMutation.data.games_count} games.
        </div>
      )}
      {refreshMutation.isError && (
        <div className="p-4 bg-red-900/50 border border-red-700 rounded-lg text-red-300">
          Failed to refresh catalogue. Please try again.
        </div>
      )}

      {/* Loading */}
      {isLoading && (
        <div className="flex justify-center py-12">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-purple-500"></div>
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="p-4 bg-red-900/50 border border-red-700 rounded-lg text-red-300">
          Failed to load games. Please try again.
        </div>
      )}

      {/* Empty State */}
      {!isLoading && !error && games.length === 0 && (
        <div className="text-center py-12">
          <p className="text-gray-400 mb-4">
            {searchQuery ? 'No games found matching your search.' : 'Your library is empty.'}
          </p>
          {!searchQuery && (
            <button
              onClick={() => refreshMutation.mutate()}
              className="px-4 py-2 bg-purple-600 hover:bg-purple-700 rounded-lg"
            >
              Refresh Catalogue
            </button>
          )}
        </div>
      )}

      {/* Game Grid */}
      {games.length > 0 && (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">
          {games.map((game) => (
            <GameCard key={game.game_id} game={game} />
          ))}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex justify-center gap-2">
          <button
            onClick={() => setPage((p) => Math.max(0, p - 1))}
            disabled={page === 0}
            className="px-4 py-2 bg-gray-800 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-lg"
          >
            Previous
          </button>
          <span className="px-4 py-2 text-gray-400">
            Page {page + 1} of {totalPages}
          </span>
          <button
            onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
            disabled={page >= totalPages - 1}
            className="px-4 py-2 bg-gray-800 hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed rounded-lg"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}

function GameCard({ game }: { game: Game }) {
  return (
    <Link
      to={`/game/${game.game_id}`}
      className="group bg-gray-800 rounded-lg overflow-hidden hover:ring-2 hover:ring-purple-500 transition-all"
    >
      {/* Cover Image */}
      <div className="aspect-[3/4] bg-gray-700 relative">
        {game.cover_image ? (
          <img
            src={game.cover_image}
            alt={game.title}
            className="w-full h-full object-cover"
            loading="lazy"
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center text-gray-500">
            No Image
          </div>
        )}
        {/* Downloaded Badge */}
        {game.is_downloaded && (
          <div className="absolute top-2 right-2" title="Downloaded">
            <CheckCircle size={20} className="text-green-400 drop-shadow-lg" fill="rgba(0,0,0,0.5)" />
          </div>
        )}
        {/* Platform Icons */}
        <div className="absolute bottom-2 left-2 flex gap-1">
          {game.platforms.windows && (
            <span className="p-1 bg-black/70 rounded" title="Windows">
              <Monitor size={14} />
            </span>
          )}
          {game.platforms.mac && (
            <span className="p-1 bg-black/70 rounded" title="macOS">
              <Apple size={14} />
            </span>
          )}
          {game.platforms.linux && (
            <span className="p-1 bg-black/70 rounded" title="Linux">
              <Terminal size={14} />
            </span>
          )}
        </div>
      </div>
      {/* Title */}
      <div className="p-3">
        <h3 className="font-medium text-sm truncate group-hover:text-purple-400 transition-colors">
          {game.title}
        </h3>
        <div className="flex gap-2 mt-1">
          {game.is_downloaded && (
            <span className="text-xs px-1.5 py-0.5 bg-green-900/50 text-green-300 rounded">
              Downloaded
            </span>
          )}
          {game.has_dlc && (
            <span className="text-xs px-1.5 py-0.5 bg-blue-900/50 text-blue-300 rounded">DLC</span>
          )}
          {game.has_extras && (
            <span className="text-xs px-1.5 py-0.5 bg-green-900/50 text-green-300 rounded">
              Extras
            </span>
          )}
        </div>
      </div>
    </Link>
  );
}
