import { Link } from 'react-router';

import { AuthControl } from '@/components/auth/AuthControl';

// Minimal top nav. Logo on the left, auth control on the right.
export function Navbar(): React.ReactNode {
  return (
    <header className="sticky top-0 z-40 w-full border-b border-border bg-background/80 backdrop-blur">
      <div className="mx-auto flex h-14 max-w-7xl items-center justify-between px-6">
        <Link
          to="/"
          className="text-sm font-semibold tracking-tight text-foreground hover:text-foreground/90"
        >
          factorbacktest
        </Link>
        <AuthControl />
      </div>
    </header>
  );
}
