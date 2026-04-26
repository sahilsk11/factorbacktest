import { Link } from 'react-router';

import { Button } from '@/components/ui/button';

// Minimal top nav. Logo on the left, placeholder Sign in on the right.
// Auth state, sign-in modal, and user menu land with the first
// protected page (see plans/frontend-v2-north-star.md §4).
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
        <Button
          variant="outline"
          size="sm"
          // Placeholder — auth wires up in a later PR.
          disabled
          aria-label="Sign in (coming soon)"
        >
          Sign in
        </Button>
      </div>
    </header>
  );
}
