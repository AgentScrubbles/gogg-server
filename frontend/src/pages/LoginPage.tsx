import { useState, useEffect } from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../App';
import { getLoginURL } from '../api/client';

export default function LoginPage() {
  const { isAuthenticated, isLoading, login } = useAuth();
  const [step, setStep] = useState<'initial' | 'waiting' | 'enter-code'>('initial');
  const [code, setCode] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [loginWindow, setLoginWindow] = useState<Window | null>(null);

  // Check if popup was closed
  useEffect(() => {
    if (!loginWindow) return;

    const interval = setInterval(() => {
      if (loginWindow.closed) {
        setLoginWindow(null);
        if (step === 'waiting') {
          setStep('enter-code');
        }
      }
    }, 500);

    return () => clearInterval(interval);
  }, [loginWindow, step]);

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
    setError(null);

    try {
      const { url } = await getLoginURL();
      // Open GOG login in a popup
      const popup = window.open(
        url,
        'GOG Login',
        'width=800,height=700,scrollbars=yes'
      );

      if (popup) {
        setLoginWindow(popup);
        setStep('waiting');
      } else {
        setError('Popup was blocked. Please allow popups for this site.');
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to get login URL');
    }
  };

  // Extract code from URL or return as-is if it's just the code
  const extractCode = (input: string): string => {
    const trimmed = input.trim();

    // Check if it looks like a URL
    if (trimmed.includes('code=')) {
      try {
        const url = new URL(trimmed);
        const codeParam = url.searchParams.get('code');
        if (codeParam) return codeParam;
      } catch {
        // Not a valid URL, try regex fallback
        const match = trimmed.match(/[?&]code=([^&]+)/);
        if (match) return match[1];
      }
    }

    // Return as-is (assume it's the raw code)
    return trimmed;
  };

  const handleCodeSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    const extractedCode = extractCode(code);
    if (!extractedCode) {
      setError('Please enter the code or URL');
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      await login(extractedCode);
      // Navigation happens automatically via isAuthenticated check
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to authenticate');
      setIsSubmitting(false);
    }
  };

  const handleReset = () => {
    setStep('initial');
    setCode('');
    setError(null);
    if (loginWindow && !loginWindow.closed) {
      loginWindow.close();
    }
    setLoginWindow(null);
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-gray-900 to-gray-800">
      <div className="max-w-md w-full p-8 bg-gray-800 rounded-xl shadow-2xl">
        <div className="text-center mb-8">
          <h1 className="text-4xl font-bold text-purple-400 mb-2">GOGG</h1>
          <p className="text-gray-400">GOG Game Downloader</p>
        </div>

        {error && (
          <div className="mb-6 p-4 bg-red-900/50 border border-red-700 rounded-lg text-red-300 text-sm">
            {error}
          </div>
        )}

        {step === 'initial' && (
          <div className="space-y-6">
            <p className="text-gray-300 text-center">
              Sign in with your GOG.com account to access your game library and download games.
            </p>

            <button
              onClick={handleLogin}
              className="w-full py-3 px-4 bg-purple-600 hover:bg-purple-700 text-white font-medium rounded-lg transition-colors"
            >
              Sign in with GOG
            </button>

            <p className="text-xs text-gray-500 text-center">
              A popup will open for you to log in to GOG.com
            </p>
          </div>
        )}

        {step === 'waiting' && (
          <div className="space-y-6">
            <div className="text-center">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-purple-500 mx-auto mb-4"></div>
              <p className="text-gray-300">
                Waiting for you to log in...
              </p>
              <p className="text-gray-500 text-sm mt-2">
                Complete the login in the popup window
              </p>
            </div>

            <button
              onClick={() => setStep('enter-code')}
              className="w-full py-2 px-4 bg-gray-700 hover:bg-gray-600 text-gray-300 text-sm rounded-lg transition-colors"
            >
              I've logged in, enter code manually
            </button>

            <button
              onClick={handleReset}
              className="w-full py-2 px-4 text-gray-500 hover:text-gray-300 text-sm transition-colors"
            >
              Cancel
            </button>
          </div>
        )}

        {step === 'enter-code' && (
          <form onSubmit={handleCodeSubmit} className="space-y-6">
            <div>
              <p className="text-gray-300 mb-4">
                After logging in, copy the <span className="text-purple-400 font-medium">entire URL</span> from your browser and paste it below:
              </p>
              <div className="p-3 bg-gray-900 rounded-lg text-xs text-gray-400 font-mono break-all mb-4">
                https://embed.gog.com/on_login_success?...&code=abc123...
              </div>
            </div>

            <input
              type="text"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="Paste the URL or code here"
              className="w-full px-4 py-3 bg-gray-900 border border-gray-700 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-purple-500"
              autoFocus
            />

            <button
              type="submit"
              disabled={isSubmitting || !code.trim()}
              className="w-full py-3 px-4 bg-purple-600 hover:bg-purple-700 disabled:bg-purple-800 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-colors flex items-center justify-center gap-2"
            >
              {isSubmitting ? (
                <>
                  <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-white"></div>
                  Authenticating...
                </>
              ) : (
                'Continue'
              )}
            </button>

            <button
              type="button"
              onClick={handleReset}
              className="w-full py-2 px-4 text-gray-500 hover:text-gray-300 text-sm transition-colors"
            >
              Start over
            </button>
          </form>
        )}
      </div>
    </div>
  );
}
