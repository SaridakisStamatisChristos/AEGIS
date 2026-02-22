import { Link, useLocation } from 'react-router-dom';
import { Shield, Activity, CheckCircle, Archive, Settings, LayoutDashboard } from 'lucide-react';

const navigation = [
  { name: 'Dashboard', href: '/dashboard', icon: LayoutDashboard },
  { name: 'Runs', href: '/runs', icon: Activity },
  { name: 'Policies', href: '/policies', icon: Shield },
  { name: 'Approvals', href: '/approvals', icon: CheckCircle },
  { name: 'Evidence', href: '/evidence', icon: Archive },
];

export default function Header() {
  const location = useLocation();

  return (
    <header className="bg-white shadow-sm border-b border-gray-200">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between h-16">
          <div className="flex">
            <div className="flex-shrink-0 flex items-center">
              <Shield className="h-8 w-8 text-blue-600" />
              <span className="ml-2 text-xl font-bold text-gray-900">AegisRun</span>
            </div>
            <nav className="ml-10 flex space-x-8">
              {navigation.map((item) => {
                const isActive = location.pathname.startsWith(item.href);
                return (
                  <Link
                    key={item.name}
                    to={item.href}
                    className={`inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium ${
                      isActive
                        ? 'border-blue-500 text-gray-900'
                        : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700'
                    }`}
                  >
                    <item.icon className="h-4 w-4 mr-2" />
                    {item.name}
                  </Link>
                );
              })}
            </nav>
          </div>
          <div className="flex items-center">
            <Link
              to="/settings"
              className="p-2 text-gray-500 hover:text-gray-700 rounded-md"
            >
              <Settings className="h-5 w-5" />
            </Link>
          </div>
        </div>
      </div>
    </header>
  );
}
