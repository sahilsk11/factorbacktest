import { Navigate } from 'react-router';
import { useAuth } from '@/lib/auth-context';

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { status, isAuthenticated } = useAuth();

  if (status === 'loading') {
    return null;
  }

  if (!isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  return children;
}