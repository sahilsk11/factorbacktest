import { type HTMLAttributes, forwardRef } from 'react';

import { cn } from '@/lib/utils';

// Minimal Card primitive. Defaults to bg-card with a 1px hairline
// border that lifts on hover (set by the consumer when interactive).
// No drop shadows — flat surfaces are part of the aesthetic.
export const Card = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(function Card(
  { className, ...props },
  ref,
) {
  return (
    <div
      ref={ref}
      className={cn('rounded-lg border border-border bg-card', className)}
      {...props}
    />
  );
});
