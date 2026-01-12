import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useQuery, useMutation } from '@tanstack/react-query';
import { ArrowLeft, Download, Monitor, Apple, Terminal, Check } from 'lucide-react';
import { getGame, createDownload } from '../api/client';
import type { CreateDownloadRequest } from '../types';

const PLATFORMS = [
  { id: 'windows', label: 'Windows', icon: Monitor },
  { id: 'mac', label: 'macOS', icon: Apple },
  { id: 'linux', label: 'Linux', icon: Terminal },
] as const;

const LANGUAGES = [
  { code: 'en', label: 'English' },
  { code: 'fr', label: 'Français' },
  { code: 'de', label: 'Deutsch' },
  { code: 'es', label: 'Español' },
  { code: 'it', label: 'Italiano' },
  { code: 'ru', label: 'Русский' },
  { code: 'pl', label: 'Polski' },
  { code: 'pt-BR', label: 'Português' },
  { code: 'zh-Hans', label: '简体中文' },
  { code: 'ja', label: '日本語' },
  { code: 'ko', label: '한국어' },
];

export default function GameDetailPage() {
  const { gameId } = useParams<{ gameId: string }>();
  const [platform, setPlatform] = useState('windows');
  const [language, setLanguage] = useState('en');
  const [includeExtras, setIncludeExtras] = useState(true);
  const [includeDLC, setIncludeDLC] = useState(true);

  const { data: game, isLoading, error } = useQuery({
    queryKey: ['game', gameId],
    queryFn: () => getGame(Number(gameId)),
    enabled: !!gameId,
  });

  const downloadMutation = useMutation({
    mutationFn: (req: CreateDownloadRequest) => createDownload(req),
  });

  const handleDownload = () => {
    if (!gameId) return;
    downloadMutation.mutate({
      game_id: Number(gameId),
      platform,
      language,
      include_extras: includeExtras,
      include_dlc: includeDLC,
      threads: 5,
    });
  };

  if (isLoading) {
    return (
      <div className="flex justify-center py-12">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-purple-500"></div>
      </div>
    );
  }

  if (error || !game) {
    return (
      <div className="text-center py-12">
        <p className="text-red-400 mb-4">Failed to load game details</p>
        <Link to="/library" className="text-purple-400 hover:text-purple-300">
          Back to Library
        </Link>
      </div>
    );
  }

  // Determine available platforms and languages
  const availablePlatforms = new Set<string>();
  const availableLanguages = new Set<string>();

  game.data.downloads.forEach((dl) => {
    availableLanguages.add(dl.language);
    if (dl.platforms.windows?.length) availablePlatforms.add('windows');
    if (dl.platforms.mac?.length) availablePlatforms.add('mac');
    if (dl.platforms.linux?.length) availablePlatforms.add('linux');
  });

  return (
    <div className="space-y-6">
      {/* Back Button */}
      <Link
        to="/library"
        className="inline-flex items-center gap-2 text-gray-400 hover:text-white transition-colors"
      >
        <ArrowLeft size={20} />
        Back to Library
      </Link>

      {/* Game Header */}
      <div className="flex gap-6">
        {/* Cover */}
        <div className="w-48 h-64 bg-gray-800 rounded-lg overflow-hidden flex-shrink-0">
          {game.data.backgroundImage ? (
            <img
              src={game.data.backgroundImage}
              alt={game.title}
              className="w-full h-full object-cover"
            />
          ) : (
            <div className="w-full h-full flex items-center justify-center text-gray-500">
              No Image
            </div>
          )}
        </div>

        {/* Info */}
        <div className="flex-1">
          <h1 className="text-3xl font-bold mb-2">{game.data.title}</h1>
          <div className="flex gap-2 mb-4">
            {availablePlatforms.has('windows') && (
              <span className="flex items-center gap-1 px-2 py-1 bg-gray-800 rounded">
                <Monitor size={16} /> Windows
              </span>
            )}
            {availablePlatforms.has('mac') && (
              <span className="flex items-center gap-1 px-2 py-1 bg-gray-800 rounded">
                <Apple size={16} /> macOS
              </span>
            )}
            {availablePlatforms.has('linux') && (
              <span className="flex items-center gap-1 px-2 py-1 bg-gray-800 rounded">
                <Terminal size={16} /> Linux
              </span>
            )}
          </div>
          <div className="flex gap-2">
            {game.data.dlcs.length > 0 && (
              <span className="text-sm px-2 py-1 bg-blue-900/50 text-blue-300 rounded">
                {game.data.dlcs.length} DLC{game.data.dlcs.length !== 1 ? 's' : ''}
              </span>
            )}
            {game.data.extras.length > 0 && (
              <span className="text-sm px-2 py-1 bg-green-900/50 text-green-300 rounded">
                {game.data.extras.length} Extra{game.data.extras.length !== 1 ? 's' : ''}
              </span>
            )}
          </div>
        </div>
      </div>

      {/* Download Options */}
      <div className="bg-gray-800 rounded-lg p-6 space-y-6">
        <h2 className="text-xl font-semibold">Download Options</h2>

        {/* Platform Selection */}
        <div>
          <label className="block text-sm font-medium text-gray-400 mb-2">Platform</label>
          <div className="flex gap-2">
            {PLATFORMS.map((p) => {
              const Icon = p.icon;
              const isAvailable = availablePlatforms.has(p.id);
              const isSelected = platform === p.id;
              return (
                <button
                  key={p.id}
                  onClick={() => isAvailable && setPlatform(p.id)}
                  disabled={!isAvailable}
                  className={`flex items-center gap-2 px-4 py-2 rounded-lg transition-colors ${
                    isSelected
                      ? 'bg-purple-600 text-white'
                      : isAvailable
                        ? 'bg-gray-700 hover:bg-gray-600 text-gray-300'
                        : 'bg-gray-900 text-gray-600 cursor-not-allowed'
                  }`}
                >
                  <Icon size={18} />
                  {p.label}
                </button>
              );
            })}
          </div>
        </div>

        {/* Language Selection */}
        <div>
          <label className="block text-sm font-medium text-gray-400 mb-2">Language</label>
          <select
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
            className="w-full max-w-xs px-4 py-2 bg-gray-700 border border-gray-600 rounded-lg focus:outline-none focus:border-purple-500"
          >
            {LANGUAGES.filter((l) => availableLanguages.has(l.label)).map((l) => (
              <option key={l.code} value={l.code}>
                {l.label}
              </option>
            ))}
          </select>
        </div>

        {/* Options */}
        <div className="flex gap-6">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={includeExtras}
              onChange={(e) => setIncludeExtras(e.target.checked)}
              className="w-5 h-5 rounded border-gray-600 bg-gray-700 text-purple-600 focus:ring-purple-500"
            />
            <span>Include Extras</span>
            {game.data.extras.length > 0 && (
              <span className="text-gray-500 text-sm">({game.data.extras.length})</span>
            )}
          </label>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={includeDLC}
              onChange={(e) => setIncludeDLC(e.target.checked)}
              className="w-5 h-5 rounded border-gray-600 bg-gray-700 text-purple-600 focus:ring-purple-500"
            />
            <span>Include DLCs</span>
            {game.data.dlcs.length > 0 && (
              <span className="text-gray-500 text-sm">({game.data.dlcs.length})</span>
            )}
          </label>
        </div>

        {/* Download Button */}
        <div className="pt-4 border-t border-gray-700">
          {downloadMutation.isSuccess ? (
            <div className="flex items-center gap-2 text-green-400">
              <Check size={20} />
              Download queued! Check the Downloads page for progress.
            </div>
          ) : downloadMutation.isError ? (
            <div className="text-red-400 mb-4">
              Failed to queue download. Please try again.
            </div>
          ) : null}

          <button
            onClick={handleDownload}
            disabled={downloadMutation.isPending || downloadMutation.isSuccess}
            className="flex items-center gap-2 px-6 py-3 bg-purple-600 hover:bg-purple-700 disabled:bg-purple-800 disabled:cursor-not-allowed rounded-lg font-medium transition-colors"
          >
            {downloadMutation.isPending ? (
              <>
                <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-white"></div>
                Queueing...
              </>
            ) : downloadMutation.isSuccess ? (
              <>
                <Check size={20} />
                Queued
              </>
            ) : (
              <>
                <Download size={20} />
                Download Game
              </>
            )}
          </button>
        </div>
      </div>

      {/* Extras List */}
      {game.data.extras.length > 0 && (
        <div className="bg-gray-800 rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-4">Extras</h2>
          <ul className="space-y-2">
            {game.data.extras.map((extra, i) => (
              <li key={i} className="flex justify-between items-center py-2 border-b border-gray-700 last:border-0">
                <span>{extra.name}</span>
                <span className="text-gray-400 text-sm">{extra.size}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* DLCs List */}
      {game.data.dlcs.length > 0 && (
        <div className="bg-gray-800 rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-4">DLCs</h2>
          <ul className="space-y-2">
            {game.data.dlcs.map((dlc, i) => (
              <li key={i} className="py-2 border-b border-gray-700 last:border-0">
                <span className="font-medium">{dlc.title}</span>
                {dlc.extras.length > 0 && (
                  <span className="ml-2 text-sm text-gray-400">
                    ({dlc.extras.length} extras)
                  </span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
