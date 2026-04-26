import { Section } from './Section';
import type { RebalanceInterval } from './types';
import { cn } from '@/lib/utils';

const OPTIONS: { value: RebalanceInterval; label: string }[] = [
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'monthly', label: 'Monthly' },
  { value: 'yearly', label: 'Yearly' },
];

export function RebalanceSection({
  value,
  setValue,
}: {
  value: RebalanceInterval;
  setValue: (v: RebalanceInterval) => void;
}): React.ReactNode {
  return (
    <Section
      step={4}
      title="Set the rebalance cadence"
      hint="How often the portfolio re-scores and swaps holdings."
    >
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        {OPTIONS.map((o) => {
          const isActive = o.value === value;
          return (
            <button
              key={o.value}
              type="button"
              onClick={() => setValue(o.value)}
              className={cn(
                'rounded-md border px-3 py-2 text-sm font-medium transition-colors',
                isActive
                  ? 'border-accent bg-accent/15 text-foreground'
                  : 'border-border bg-elevated/50 text-muted-foreground hover:border-border-strong hover:text-foreground',
              )}
            >
              {o.label}
            </button>
          );
        })}
      </div>
    </Section>
  );
}
