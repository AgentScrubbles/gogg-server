export interface User {
  id: number;
  username: string;
  gog_user_id?: string;
  created_at: string;
  updated_at: string;
}

export interface Game {
  id: number;
  game_id: number;
  title: string;
  cover_image?: string;
  background_image?: string;
  has_dlc: boolean;
  has_extras: boolean;
  is_downloaded: boolean;
  platforms: {
    windows?: boolean;
    mac?: boolean;
    linux?: boolean;
  };
  created_at: string;
  updated_at: string;
}

export interface GameDetail {
  id: number;
  game_id: number;
  title: string;
  data: {
    title: string;
    backgroundImage?: string;
    downloads: Download[];
    extras: Extra[];
    dlcs: DLC[];
  };
  created_at: string;
  updated_at: string;
}

export interface Download {
  language: string;
  platforms: {
    windows?: PlatformFile[];
    mac?: PlatformFile[];
    linux?: PlatformFile[];
  };
}

export interface PlatformFile {
  name: string;
  version?: string;
  date?: string;
  size: string;
  manualUrl?: string;
}

export interface Extra {
  name: string;
  size: string;
  manualUrl: string;
}

export interface DLC {
  title: string;
  backgroundImage?: string;
  downloads: Download[];
  extras: Extra[];
}

export type DownloadStatus = 'pending' | 'downloading' | 'completed' | 'failed' | 'cancelled';

export interface DownloadJob {
  id: number;
  user_id: number;
  game_id: number;
  game_title: string;
  status: DownloadStatus;
  platform: string;
  language: string;
  include_extras: boolean;
  include_dlc: boolean;
  threads: number;
  progress_bytes: number;
  total_bytes: number;
  speed_bytes_per_sec?: number;
  eta_seconds?: number;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface CreateDownloadRequest {
  game_id: number;
  platform: string;
  language: string;
  include_extras: boolean;
  include_dlc: boolean;
  threads: number;
}
