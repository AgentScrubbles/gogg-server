import { useState, useEffect, useCallback, useRef } from 'react';
import { connectDownloadProgress } from '../api/client';
import type { DownloadJob, DownloadStatus } from '../types';

interface ProgressUpdate {
  job_id: number;
  progress_bytes: number;
  total_bytes: number;
  status: DownloadStatus;
  speed_bytes_per_sec?: number;
  eta_seconds?: number;
}

interface WSMessage {
  type: 'initial_state' | 'progress' | 'job_update';
  data: DownloadJob[] | ProgressUpdate | DownloadJob;
}

export function useDownloadProgress() {
  const [activeDownloads, setActiveDownloads] = useState<Map<number, DownloadJob>>(new Map());
  const [isConnected, setIsConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);

  const handleMessage = useCallback((message: WSMessage) => {
    switch (message.type) {
      case 'initial_state':
        const jobs = message.data as DownloadJob[];
        const jobMap = new Map<number, DownloadJob>();
        jobs.forEach((job) => jobMap.set(job.id, job));
        setActiveDownloads(jobMap);
        break;

      case 'progress':
        const progress = message.data as ProgressUpdate;
        setActiveDownloads((prev) => {
          const updated = new Map(prev);
          const job = updated.get(progress.job_id);
          if (job) {
            updated.set(progress.job_id, {
              ...job,
              progress_bytes: progress.progress_bytes,
              status: progress.status,
              speed_bytes_per_sec: progress.speed_bytes_per_sec,
              eta_seconds: progress.eta_seconds,
            });
          }
          return updated;
        });
        break;

      case 'job_update':
        const updatedJob = message.data as DownloadJob;
        setActiveDownloads((prev) => {
          const updated = new Map(prev);
          if (updatedJob.status === 'completed' || updatedJob.status === 'failed' || updatedJob.status === 'cancelled') {
            updated.delete(updatedJob.id);
          } else {
            updated.set(updatedJob.id, updatedJob);
          }
          return updated;
        });
        break;
    }
  }, []);

  const connect = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN) return;

    wsRef.current = connectDownloadProgress(
      (data) => handleMessage(data as WSMessage),
      () => {
        setIsConnected(false);
        // Attempt to reconnect after 5 seconds
        setTimeout(connect, 5000);
      }
    );

    if (wsRef.current) {
      wsRef.current.onopen = () => setIsConnected(true);
      wsRef.current.onclose = () => {
        setIsConnected(false);
        // Attempt to reconnect after 5 seconds
        setTimeout(connect, 5000);
      };
    }
  }, [handleMessage]);

  const disconnect = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setIsConnected(false);
  }, []);

  useEffect(() => {
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  return {
    activeDownloads: Array.from(activeDownloads.values()),
    isConnected,
    connect,
    disconnect,
  };
}
