import { Outlet } from 'react-router';

import { Navbar } from './Navbar';

// Page shell wrapping every route. Top nav + main content area.
// Future: footer, mobile drawer.
export function RootLayout(): React.ReactNode {
  return (
    <div className="flex min-h-full flex-col bg-background text-foreground">
      <Navbar />
      <main className="flex-1">
        <Outlet />
      </main>
    </div>
  );
}
