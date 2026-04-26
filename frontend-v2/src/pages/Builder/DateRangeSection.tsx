import { Section } from './Section';
import { cn } from '@/lib/utils';

const MIN_DATE = '2010-01-01';

interface Preset {
  label: string;
  days: number;
}

const PRESETS: Preset[] = [
  { label: '30d', days: 30 },
  { label: '90d', days: 90 },
  { label: '1y', days: 365 },
  { label: '3y', days: 365 * 3 },
];

function daysAgo(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return d.toISOString().split('T')[0];
}

export function DateRangeSection({
  start,
  setStart,
  end,
  setEnd,
}: {
  start: string;
  setStart: (v: string) => void;
  end: string;
  setEnd: (v: string) => void;
}): React.ReactNode {
  const today = new Date().toISOString().split('T')[0];
  const startMs = new Date(start).getTime();
  const endMs = new Date(end).getTime();
  const days = Math.max(0, (endMs - startMs) / (1000 * 60 * 60 * 24));

  // Detect which preset (if any) the current range matches, so the
  // right chip stays highlighted when the user clicks one.
  const activePreset = PRESETS.find((p) => {
    const expectedStart = daysAgo(p.days);
    return start === expectedStart && end === today;
  });

  function applyPreset(p: Preset) {
    setStart(daysAgo(p.days));
    setEnd(today);
  }

  function formatDuration(d: number): string {
    if (d >= 365) return `${(d / 365).toFixed(1)} yrs`;
    return `${Math.round(d)} days`;
  }

  return (
    <Section
      step={3}
      title="Set the backtest range"
      hint="History to simulate over. Longer windows capture more market regimes."
    >
      <div className="mb-4 flex flex-wrap gap-2">
        {PRESETS.map((p) => {
          const active = activePreset?.label === p.label;
          return (
            <button
              key={p.label}
              type="button"
              onClick={() => applyPreset(p)}
              className={cn(
                'rounded-md border px-3 py-1.5 font-mono text-sm transition-colors',
                active
                  ? 'border-accent bg-accent/15 text-foreground'
                  : 'border-border bg-elevated/50 text-muted-foreground hover:border-border-strong hover:text-foreground',
              )}
            >
              {p.label}
            </button>
          );
        })}
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <DateInput
          label="Start"
          value={start}
          onChange={setStart}
          min={MIN_DATE}
          max={end > today ? today : end}
        />
        <span className="pt-5 text-sm text-muted-foreground">to</span>
        <DateInput label="End" value={end} onChange={setEnd} min={start} max={today} />
        <p className="pt-5 font-mono text-sm text-muted-foreground">
          {formatDuration(days)}
        </p>
      </div>
    </Section>
  );
}

function DateInput({
  label,
  value,
  onChange,
  min,
  max,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  min: string;
  max: string;
}): React.ReactNode {
  return (
    <label className="flex flex-col">
      <span className="text-xs font-medium tracking-wide text-subtle-foreground uppercase">
        {label}
      </span>
      <input
        type="date"
        value={value}
        min={min}
        max={max}
        onChange={(e) => onChange(e.target.value)}
        className={cn(
          'mt-1 h-10 rounded-md border border-border bg-elevated/70 px-3 font-mono text-sm text-foreground',
          'focus:border-border-strong focus:bg-elevated focus:outline-none',
          '[color-scheme:dark]',
        )}
      />
    </label>
  );
}
