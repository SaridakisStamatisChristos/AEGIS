import { Link, useLocation } from 'react-router-dom';
import { 
  LayoutDashboard, 
  Activity, 
  Shield, 
  CheckCircle, 
  Archive,
  Settings,
  ChevronRight 
} from 'lucide-react';

const navigation = [
  { name: 'Dashboard', href: '/dashboard', icon: LayoutDashboard },
  { name: 'Runs', href: '/runs', icon: Activity },
  { name: 'Policies', href: '/policies', icon: Shield },
  { name: 'Approvals', href: '/approvals', icon: CheckCircle },
  { name: 'Evidence', href: '/evidence', icon: Archive },
  { name: 'Settings', href: '/settings', icon: Settings },
];

export default function Sidebar() {
  const location = useLocation();

  return (
    <aside className="w-64 bg-white border-r border-gray-200 min-h-screen">
      <div className="p-4">
        <div className="flex items-center mb-8">
          <Shield className="h-8 w-8 text-blue-600" />
          <span className="ml-2 text-xl font-bold text-gray-900">AegisRun</span>
        </div>
        <nav className="space-y-1">
          {navigation.map((item) => {
            const isActive = location.pathname === item.href || 
                           location.pathname.startsWith(item.href + '/');
            return (
              <Link
                key={item.name}
                to={item.href}
                className={`flex items-center px-3 py-2 text-sm font-medium rounded-md ${
                  isActive
                    ? 'bg-blue-50 text-blue-700'
                    : 'text-gray-700 hover:bg-gray-50 hover:text-gray-900'
                }`}
              >
                <item.icon className={`h-5 w-5 mr-3 ${
                  isActive ? 'text-blue-500' : 'text-gray-400'
                }`} />
                {item.name}
                {isActive && (
                  <ChevronRight className="ml-auto h-4 w-4 text-blue-500" />
                )}
              </Link>
            );
          })}
        </nav>
      </div>
    </aside>
  );
}
