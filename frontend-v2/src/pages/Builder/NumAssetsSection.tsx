import { Section } from './Section';
import { formatNumber } from '@/lib/format';
import { cn } from '@/lib/utils';

export function NumAssetsSection({
  value,
  setValue,
  universeSize,
}: {
  value: number;
  setValue: (n: number) => void;
  universeSize: number;
}): React.ReactNode {
  const max = Math.max(3, universeSize || 500);
  const safeValue = Math.min(Math.max(value, 3), max);

  return (
    <Section
      step={5}
      title="How many assets to hold"
      hint="The top N scoring names get held at each rebalance. Lower is more concentrated."
    >
      <div className="flex items-center gap-4">
        <input
          type="range"
          min={3}
          max={max}
          step={1}
          value={safeValue}
          onChange={(e) => setValue(parseInt(e.target.value, 10))}
          style={{ accentColor: 'var(--color-accent)' }}
          className="h-2 flex-1 cursor-pointer"
        />
        <input
          type="number"
          min={3}
          max={max}
          value={safeValue}
          onChange={(e) => {
            const v = parseInt(e.target.value, 10);
            if (Number.isFinite(v)) setValue(v);
          }}
          className={cn(
            'h-10 w-20 rounded-md border border-border bg-elevated/70 px-3 text-center font-mono text-sm',
            'focus:border-border-strong focus:bg-elevated focus:outline-none',
          )}
        />
      </div>
      {universeSize > 0 && (
        <p className="mt-3 text-sm text-muted-foreground">
          Top{' '}
          <span className="font-mono text-foreground">
            {((safeValue / universeSize) * 100).toFixed(0)}%
          </span>{' '}
          of the{' '}
          <span className="font-mono text-foreground">{formatNumber(universeSize)}</span>-asset
          universe by score.
        </p>
      )}
    </Section>
  );
}
