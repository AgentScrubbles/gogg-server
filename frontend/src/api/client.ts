import type {
  User,
  Game,
  GameDetail,
  DownloadJob,
  PaginatedResponse,
  CreateDownloadRequest,
  DownloadStatus,
} from '../types';

const API_BASE = '/api';

class APIError extends Error {
  constructor(public status: number, public code: string, message: string) {
    super(message);
    this.name = 'APIError';
  }
}

async function request<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const token = localStorage.getItem('token');

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  // Merge any additional headers from options
  if (options.headers) {
    Object.assign(headers, options.headers);
  }

  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ code: 'unknown', message: 'Unknown error' }));
    throw new APIError(response.status, error.code, error.message);
  }

  return response.json();
}

// Auth
export async function getLoginURL(): Promise<{ url: string }> {
  return request('/auth/login-url');
}

export async function exchangeCode(code: string, username?: string): Promise<{ token: string; user: User }> {
  return request('/auth/exchange', {
    method: 'POST',
    body: JSON.stringify({ code, username }),
  });
}

export async function logout(): Promise<void> {
  await request('/auth/logout', { method: 'POST' });
  localStorage.removeItem('token');
}

export async function getCurrentUser(): Promise<User> {
  return request('/auth/me');
}

// Games
export async function getGames(limit = 50, offset = 0): Promise<PaginatedResponse<Game>> {
  return request(`/games?limit=${limit}&offset=${offset}`);
}

export async function searchGames(query: string, limit = 50, offset = 0): Promise<PaginatedResponse<Game>> {
  return request(`/games/search?q=${encodeURIComponent(query)}&limit=${limit}&offset=${offset}`);
}

export async function getGame(gameId: number): Promise<GameDetail> {
  return request(`/games/${gameId}`);
}

export async function refreshCatalogue(): Promise<{ message: string; games_count: number }> {
  return request('/games/refresh', { method: 'POST' });
}

// Downloads
export async function getDownloads(
  status?: DownloadStatus,
  limit = 50,
  offset = 0
): Promise<PaginatedResponse<DownloadJob>> {
  let url = `/downloads?limit=${limit}&offset=${offset}`;
  if (status) {
    url += `&status=${status}`;
  }
  return request(url);
}

export async function createDownload(req: CreateDownloadRequest): Promise<DownloadJob> {
  return request('/downloads', {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

export async function getDownload(downloadId: number): Promise<DownloadJob> {
  return request(`/downloads/${downloadId}`);
}

export async function cancelDownload(downloadId: number): Promise<void> {
  await request(`/downloads/${downloadId}`, { method: 'DELETE' });
}

// WebSocket
export function connectDownloadProgress(
  onMessage: (data: unknown) => void,
  onError?: (error: Event) => void
): WebSocket | null {
  const token = localStorage.getItem('token');
  if (!token) return null;

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(`${protocol}//${window.location.host}/api/ws/downloads?token=${token}`);

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      onMessage(data);
    } catch (e) {
      console.error('Failed to parse WebSocket message:', e);
    }
  };

  ws.onerror = (error) => {
    console.error('WebSocket error:', error);
    onError?.(error);
  };

  return ws;
}

export { APIError };
