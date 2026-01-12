import { useState } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../App';
import { getLoginURL } from '../api/client';

export default function LoginPage() {
  const { isAuthenticated, isLoading } = useAuth();
  const [isRedirecting, setIsRedirecting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-purple-500"></div>
      </div>
    );
  }

  if (isAuthenticated) {
    return <Navigate to="/library" replace />;
  }

  const handleLogin = async () => {
    setIsRedirecting(true);
    setError(null);

    try {
      const { url } = await getLoginURL();
      // Open GOG login in same window - GOG will redirect back to our callback
      window.location.href = url;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to get login URL');
      setIsRedirecting(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-900 to-gray-800">
      <div className="max-w-md w-full p-8 bg-gray-800 rounded-xl shadow-2xl">
        <div className="text-center mb-8">
          <h1 className="text-4xl font-bold text-purple-400 mb-2">GOGG</h1>
          <p className="text-gray-400">GOG Game Downloader</p>
        </div>

        <div className="space-y-6">
          <p className="text-gray-300 text-center">
            Sign in with your GOG.com account to access your game library and download games.
          </p>

          {error && (
            <div className="p-4 bg-red-900/50 border border-red-700 rounded-lg text-red-300 text-sm">
              {error}
            </div>
          )}

          <button
            onClick={handleLogin}
            disabled={isRedirecting}
            className="w-full py-3 px-4 bg-purple-600 hover:bg-purple-700 disabled:bg-purple-800 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-colors flex items-center justify-center gap-2"
          >
            {isRedirecting ? (
              <>
                <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-white"></div>
                Redirecting to GOG...
              </>
            ) : (
              'Sign in with GOG'
            )}
          </button>

          <p className="text-xs text-gray-500 text-center">
            By signing in, you agree to allow this application to access your GOG.com account to
            manage your game library.
          </p>
        </div>
      </div>
    </div>
  );
}
