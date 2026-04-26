import { type InputHTMLAttributes, forwardRef } from 'react';

import { cn } from '@/lib/utils';

export type InputProps = InputHTMLAttributes<HTMLInputElement>;

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, ...props },
  ref,
) {
  return (
    <input
      ref={ref}
      className={cn(
        'h-11 w-full rounded-md border border-border bg-elevated/70 px-3 text-sm text-foreground',
        'placeholder:text-subtle-foreground',
        'transition-[border-color,background-color] duration-150 ease-out',
        'focus:border-border-strong focus:bg-elevated focus:outline-none',
        'disabled:cursor-not-allowed disabled:opacity-50',
        className,
      )}
      {...props}
    />
  );
});
