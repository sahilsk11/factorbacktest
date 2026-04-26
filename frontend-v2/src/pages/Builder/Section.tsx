import type { ReactNode } from 'react';

import { cn } from '@/lib/utils';

// Builder sections share a numbered label + title + helper line.
// Pulled out so every input reads with the same rhythm down the page.
export function Section({
  step,
  title,
  hint,
  children,
  className,
}: {
  step: number;
  title: string;
  hint: string;
  children: ReactNode;
  className?: string;
}): React.ReactNode {
  return (
    <section className={cn('rounded-lg border border-border bg-card p-6', className)}>
      <header className="mb-5 flex items-baseline gap-3">
        <span className="font-mono text-xs text-subtle-foreground">0{step}</span>
        <div>
          <h3 className="text-base font-semibold tracking-tight text-foreground">{title}</h3>
          <p className="mt-0.5 text-sm text-muted-foreground">{hint}</p>
        </div>
      </header>
      {children}
    </section>
  );
}
