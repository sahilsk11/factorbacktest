import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

// shadcn convention: single helper that handles conditional classnames
// (clsx) plus deduping conflicting Tailwind utilities (tailwind-merge).
// Used by every component that composes classes.
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
