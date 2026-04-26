// Placeholder for /backtest and /investments so the landing-page CTA
// and nav links don't 404 during the rewrite. Each lands as a real
// page in a subsequent PR.
export function ComingSoonPage({ pageName }: { pageName: string }): React.ReactNode {
  return (
    <div className="mx-auto flex max-w-7xl flex-col items-center justify-center px-6 py-32 text-center">
      <p className="text-xs font-medium uppercase tracking-widest text-subtle-foreground">
        Coming soon
      </p>
      <h1 className="mt-4 text-3xl font-semibold tracking-tight text-foreground">{pageName}</h1>
      <p className="mt-3 max-w-md text-sm text-muted-foreground">
        This page is part of the frontend-v2 rewrite and is being built in a follow-up PR.
      </p>
    </div>
  );
}
