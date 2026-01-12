import { ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Library, Download, LogOut } from 'lucide-react';
import { useAuth } from '../App';

interface LayoutProps {
  children: ReactNode;
}

export default function Layout({ children }: LayoutProps) {
  const { user, logout } = useAuth();
  const location = useLocation();

  const navItems = [
    { path: '/library', label: 'Library', icon: Library },
    { path: '/downloads', label: 'Downloads', icon: Download },
  ];

  return (
    <div className="min-h-screen flex">
      {/* Sidebar */}
      <aside className="w-64 bg-gray-800 border-r border-gray-700 flex flex-col">
        <div className="p-4 border-b border-gray-700">
          <h1 className="text-xl font-bold text-purple-400">GOGG</h1>
          <p className="text-sm text-gray-400">GOG Game Downloader</p>
        </div>

        <nav className="flex-1 p-4">
          <ul className="space-y-2">
            {navItems.map((item) => {
              const Icon = item.icon;
              const isActive = location.pathname.startsWith(item.path);
              return (
                <li key={item.path}>
                  <Link
                    to={item.path}
                    className={`flex items-center gap-3 px-4 py-2 rounded-lg transition-colors ${
                      isActive
                        ? 'bg-purple-600 text-white'
                        : 'text-gray-300 hover:bg-gray-700 hover:text-white'
                    }`}
                  >
                    <Icon size={20} />
                    {item.label}
                  </Link>
                </li>
              );
            })}
          </ul>
        </nav>

        <div className="p-4 border-t border-gray-700">
          <div className="flex items-center justify-between">
            <div>
              <p className="font-medium text-sm">{user?.username}</p>
              <p className="text-xs text-gray-400">Logged in</p>
            </div>
            <button
              onClick={logout}
              className="p-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded-lg transition-colors"
              title="Logout"
            >
              <LogOut size={20} />
            </button>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <div className="p-6">{children}</div>
      </main>
    </div>
  );
}
