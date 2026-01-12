import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useState, useEffect, useCallback, createContext, useContext, ReactNode } from 'react';
import { getCurrentUser, logout as apiLogout, exchangeCode as apiExchangeCode } from './api/client';
import type { User } from './types';
import LoginPage from './pages/LoginPage';
import LibraryPage from './pages/LibraryPage';
import GameDetailPage from './pages/GameDetailPage';
import DownloadsPage from './pages/DownloadsPage';
import Layout from './components/Layout';

// Auth Context
interface AuthContextType {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  login: (code: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | null>(null);

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
}

function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const token = localStorage.getItem('token');
    if (!token) {
      setIsLoading(false);
      return;
    }

    getCurrentUser()
      .then(setUser)
      .catch(() => {
        localStorage.removeItem('token');
        setUser(null);
      })
      .finally(() => setIsLoading(false));
  }, []);

  const login = useCallback(async (code: string) => {
    const { token, user: userData } = await apiExchangeCode(code);
    localStorage.setItem('token', token);
    setUser(userData);
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiLogout();
    } finally {
      localStorage.removeItem('token');
      setUser(null);
    }
  }, []);

  return (
    <AuthContext.Provider value={{ user, isLoading, isAuthenticated: !!user, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-purple-500"></div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/oauth/callback" element={<OAuthCallback />} />
          <Route
            path="/*"
            element={
              <ProtectedRoute>
                <Layout>
                  <Routes>
                    <Route path="/" element={<Navigate to="/library" replace />} />
                    <Route path="/library" element={<LibraryPage />} />
                    <Route path="/game/:gameId" element={<GameDetailPage />} />
                    <Route path="/downloads" element={<DownloadsPage />} />
                  </Routes>
                </Layout>
              </ProtectedRoute>
            }
          />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  );
}

function OAuthCallback() {
  const { login } = useAuth();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get('code');

    if (code) {
      login(code)
        .then(() => {
          window.location.href = '/library';
        })
        .catch((err) => {
          setError(err.message || 'Login failed');
        });
    } else {
      setError('No authorization code received');
    }
  }, [login]);

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-red-500 mb-4">Login Failed</h1>
          <p className="text-gray-400 mb-4">{error}</p>
          <a href="/login" className="text-purple-400 hover:text-purple-300">
            Try again
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="text-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-purple-500 mx-auto mb-4"></div>
        <p className="text-gray-400">Completing login...</p>
      </div>
    </div>
  );
}
