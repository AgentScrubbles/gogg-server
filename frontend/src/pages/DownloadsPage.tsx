import { useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { X, CheckCircle, XCircle, Clock, Download, RefreshCw } from 'lucide-react';
import { getDownloads, cancelDownload } from '../api/client';
import { useDownloadProgress } from '../hooks/useDownloads';
import type { DownloadJob, DownloadStatus } from '../types';

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function getStatusIcon(status: DownloadStatus) {
  switch (status) {
    case 'pending':
      return <Clock className="text-yellow-400" size={20} />;
    case 'downloading':
      return <Download className="text-blue-400 animate-pulse" size={20} />;
    case 'completed':
      return <CheckCircle className="text-green-400" size={20} />;
    case 'failed':
      return <XCircle className="text-red-400" size={20} />;
    case 'cancelled':
      return <X className="text-gray-400" size={20} />;
  }
}

function getStatusLabel(status: DownloadStatus): string {
  switch (status) {
    case 'pending':
      return 'Pending';
    case 'downloading':
      return 'Downloading';
    case 'completed':
      return 'Completed';
    case 'failed':
      return 'Failed';
    case 'cancelled':
      return 'Cancelled';
  }
}

export default function DownloadsPage() {
  const queryClient = useQueryClient();
  const { activeDownloads, isConnected, connect, disconnect } = useDownloadProgress();

  // Connect to WebSocket when component mounts
  useEffect(() => {
    connect();
    return () => disconnect();
  }, [connect, disconnect]);

  // Fetch all downloads
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['downloads'],
    queryFn: () => getDownloads(undefined, 100, 0),
    refetchInterval: 10000, // Refresh every 10 seconds
  });

  const cancelMutation = useMutation({
    mutationFn: cancelDownload,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['downloads'] });
    },
  });

  // Merge active downloads from WebSocket with fetched data
  const downloads = data?.data ?? [];
  const activeMap = new Map(activeDownloads.map((d) => [d.id, d]));
  const mergedDownloads = downloads.map((d) => activeMap.get(d.id) ?? d);

  // Separate active and completed
  const active = mergedDownloads.filter(
    (d) => d.status === 'pending' || d.status === 'downloading'
  );
  const history = mergedDownloads.filter(
    (d) => d.status === 'completed' || d.status === 'failed' || d.status === 'cancelled'
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Downloads</h1>
          <p className="text-gray-400">
            {isConnected ? (
              <span className="flex items-center gap-2">
                <span className="w-2 h-2 bg-green-400 rounded-full animate-pulse"></span>
                Connected - real-time updates
              </span>
            ) : (
              <span className="flex items-center gap-2">
                <span className="w-2 h-2 bg-red-400 rounded-full"></span>
                Disconnected
              </span>
            )}
          </p>
        </div>
        <button
          onClick={() => refetch()}
          className="flex items-center gap-2 px-4 py-2 bg-gray-800 hover:bg-gray-700 rounded-lg transition-colors"
        >
          <RefreshCw size={18} />
          Refresh
        </button>
      </div>

      {/* Active Downloads */}
      <div className="bg-gray-800 rounded-lg p-6">
        <h2 className="text-xl font-semibold mb-4">Active Downloads</h2>
        {isLoading ? (
          <div className="flex justify-center py-8">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-purple-500"></div>
          </div>
        ) : active.length === 0 ? (
          <p className="text-gray-400 py-4">No active downloads</p>
        ) : (
          <div className="space-y-4">
            {active.map((job) => (
              <DownloadCard
                key={job.id}
                job={job}
                onCancel={() => cancelMutation.mutate(job.id)}
                isCancelling={cancelMutation.isPending}
              />
            ))}
          </div>
        )}
      </div>

      {/* Download History */}
      <div className="bg-gray-800 rounded-lg p-6">
        <h2 className="text-xl font-semibold mb-4">History</h2>
        {history.length === 0 ? (
          <p className="text-gray-400 py-4">No download history</p>
        ) : (
          <div className="space-y-2">
            {history.map((job) => (
              <HistoryItem key={job.id} job={job} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function DownloadCard({
  job,
  onCancel,
  isCancelling,
}: {
  job: DownloadJob;
  onCancel: () => void;
  isCancelling: boolean;
}) {
  const progress = job.total_bytes > 0 ? (job.progress_bytes / job.total_bytes) * 100 : 0;

  return (
    <div className="bg-gray-900 rounded-lg p-4">
      <div className="flex items-start justify-between mb-3">
        <div>
          <Link
            to={`/game/${job.game_id}`}
            className="font-medium hover:text-purple-400 transition-colors"
          >
            {job.game_title}
          </Link>
          <div className="flex gap-2 mt-1 text-sm text-gray-400">
            <span>{job.platform}</span>
            <span>|</span>
            <span>{job.language}</span>
            {job.include_extras && <span>| Extras</span>}
            {job.include_dlc && <span>| DLC</span>}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {getStatusIcon(job.status)}
          <span className="text-sm">{getStatusLabel(job.status)}</span>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="mb-2">
        <div className="h-2 bg-gray-700 rounded-full overflow-hidden">
          <div
            className="h-full bg-purple-500 transition-all duration-300"
            style={{ width: `${progress}%` }}
          ></div>
        </div>
      </div>

      <div className="flex items-center justify-between text-sm text-gray-400">
        <span>
          {formatBytes(job.progress_bytes)} / {formatBytes(job.total_bytes)}
        </span>
        <span>{progress.toFixed(1)}%</span>
      </div>

      {/* Cancel Button */}
      {(job.status === 'pending' || job.status === 'downloading') && (
        <button
          onClick={onCancel}
          disabled={isCancelling}
          className="mt-3 px-3 py-1 text-sm text-red-400 hover:text-red-300 hover:bg-red-900/30 rounded transition-colors"
        >
          {isCancelling ? 'Cancelling...' : 'Cancel'}
        </button>
      )}
    </div>
  );
}

function HistoryItem({ job }: { job: DownloadJob }) {
  return (
    <div className="flex items-center justify-between py-3 border-b border-gray-700 last:border-0">
      <div className="flex items-center gap-3">
        {getStatusIcon(job.status)}
        <div>
          <Link
            to={`/game/${job.game_id}`}
            className="font-medium hover:text-purple-400 transition-colors"
          >
            {job.game_title}
          </Link>
          <div className="text-sm text-gray-400">
            {job.platform} | {job.language}
            {job.error_message && (
              <span className="text-red-400 ml-2">- {job.error_message}</span>
            )}
          </div>
        </div>
      </div>
      <div className="text-sm text-gray-400">
        {job.completed_at && new Date(job.completed_at).toLocaleDateString()}
      </div>
    </div>
  );
}
