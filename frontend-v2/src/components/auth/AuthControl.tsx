import { ChevronDown, LoaderCircle, LogOut, ShieldCheck, UserRound } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';

import { AuthModal } from './AuthModal';
import { Button } from '@/components/ui/button';
import { useAuth } from '@/lib/auth-context';
import { cn } from '@/lib/utils';

export function AuthControl(): React.ReactNode {
  const { status, user, signOut } = useAuth();
  const [modalOpen, setModalOpen] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);
  const [signingOut, setSigningOut] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const signOutRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!menuOpen) return;

    const trigger = triggerRef.current;
    signOutRef.current?.focus();

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMenuOpen(false);
    };

    const onPointerDown = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (triggerRef.current?.contains(target) || menuRef.current?.contains(target)) return;
      setMenuOpen(false);
    };

    window.addEventListener('keydown', onKeyDown);
    window.addEventListener('pointerdown', onPointerDown);
    return () => {
      window.removeEventListener('keydown', onKeyDown);
      window.removeEventListener('pointerdown', onPointerDown);
      trigger?.focus();
    };
  }, [menuOpen]);

  const handleSignOut = async () => {
    if (signingOut) return;
    setSigningOut(true);
    setError(null);
    try {
      await signOut();
      setMenuOpen(false);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Sign out failed');
    } finally {
      setSigningOut(false);
    }
  };

  if (status === 'loading') {
    return (
      <div
        className="h-8 w-24 animate-pulse rounded-md border border-border bg-elevated/70"
        aria-label="Loading auth state"
      />
    );
  }

  if (!user) {
    return (
      <>
        <Button variant="outline" size="sm" onClick={() => setModalOpen(true)}>
          <ShieldCheck className="size-4" aria-hidden />
          Sign in
        </Button>
        {modalOpen && <AuthModal open={modalOpen} onClose={() => setModalOpen(false)} />}
      </>
    );
  }

  return (
    <div className="relative">
      <button
        ref={triggerRef}
        type="button"
        className={cn(
          'flex h-9 items-center gap-2 rounded-md border border-border bg-elevated/50 px-2.5 text-sm font-medium',
          'transition-[border-color,background-color] hover:border-border-strong hover:bg-elevated',
        )}
        aria-expanded={menuOpen}
        aria-haspopup="menu"
        onClick={() => {
          setMenuOpen((open) => !open);
          setError(null);
        }}
      >
        <span className="grid size-6 place-items-center rounded-md bg-gain/15 text-gain">
          <UserRound className="size-3.5" aria-hidden />
        </span>
        <span className="hidden max-w-28 truncate font-mono text-xs text-foreground sm:inline">
          {shortUserId(user.id)}
        </span>
        <ChevronDown className="size-3.5 text-muted-foreground" aria-hidden />
      </button>

      {menuOpen && (
        <div
          ref={menuRef}
          className="absolute top-11 right-0 z-50 w-64 rounded-lg border border-border-strong bg-card p-2 shadow-2xl shadow-black/35"
          role="menu"
        >
          <div className="border-b border-border px-3 py-2">
            <p className="text-xs uppercase tracking-widest text-subtle-foreground">Signed in</p>
            <p className="mt-1 truncate font-mono text-sm text-foreground">{user.id}</p>
          </div>

          {error && (
            <div className="mt-2 rounded-md border border-loss/30 bg-loss/10 px-3 py-2 text-sm text-loss">
              {error}
            </div>
          )}

          <button
            ref={signOutRef}
            type="button"
            className="mt-2 flex h-10 w-full items-center justify-between rounded-md px-3 text-sm text-muted-foreground transition-colors hover:bg-elevated hover:text-foreground"
            disabled={signingOut}
            role="menuitem"
            onClick={() => {
              void handleSignOut();
            }}
          >
            <span className="flex items-center gap-2">
              <LogOut className="size-4" aria-hidden />
              Log out
            </span>
            {signingOut && <LoaderCircle className="size-4 animate-spin" aria-hidden />}
          </button>
        </div>
      )}
    </div>
  );
}

function shortUserId(id: string): string {
  if (id.length <= 8) return id;
  return `${id.slice(0, 4)}...${id.slice(-4)}`;
}
