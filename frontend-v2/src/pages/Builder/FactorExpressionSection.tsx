import { FACTOR_PRESETS, type FactorPreset } from './presets';
import { Section } from './Section';
import { cn } from '@/lib/utils';

// Step 1 — what the strategy *is*. Preset chips fill both expression
// and a derived name; the textarea remains editable underneath. We
// keep the preview parsing dumb on purpose: a hint that the engine
// reads symbols like `pe()`, `roe()`, `pricePercentChange(...)`.
export function FactorExpressionSection({
  expression,
  setExpression,
  name,
  setName,
  presetId,
  setPresetId,
}: {
  expression: string;
  setExpression: (v: string) => void;
  name: string;
  setName: (v: string) => void;
  presetId: string | null;
  setPresetId: (id: string | null) => void;
}): React.ReactNode {
  function applyPreset(p: FactorPreset) {
    setExpression(p.expression);
    setName(p.name);
    setPresetId(p.id);
  }

  return (
    <Section
      step={1}
      title="Pick a factor"
      hint="A formula that scores every asset on each rebalance day. Top scores get held."
    >
      <div className="flex flex-wrap gap-2">
        {FACTOR_PRESETS.map((p) => {
          const active = presetId === p.id;
          return (
            <button
              key={p.id}
              type="button"
              onClick={() => applyPreset(p)}
              className={cn(
                'rounded-md border px-3 py-1.5 text-sm transition-colors',
                active
                  ? 'border-accent bg-accent/15 text-foreground'
                  : 'border-border bg-elevated/50 text-muted-foreground hover:border-border-strong hover:text-foreground',
              )}
            >
              {p.name}
            </button>
          );
        })}
      </div>

      {presetId && (
        <p className="mt-3 text-sm text-muted-foreground">
          {FACTOR_PRESETS.find((p) => p.id === presetId)?.blurb}
        </p>
      )}

      <div className="mt-5 space-y-3">
        <label className="block">
          <span className="text-xs font-medium tracking-wide text-subtle-foreground uppercase">
            Expression
          </span>
          <textarea
            value={expression}
            onChange={(e) => {
              setExpression(e.target.value);
              setPresetId(null);
            }}
            rows={2}
            spellCheck={false}
            className={cn(
              'mt-1 block w-full resize-none rounded-md border border-border bg-elevated/70 px-3 py-2',
              'font-mono text-sm text-foreground',
              'transition-[border-color,background-color] duration-150 ease-out',
              'focus:border-border-strong focus:bg-elevated focus:outline-none',
            )}
          />
        </label>
        <label className="block">
          <span className="text-xs font-medium tracking-wide text-subtle-foreground uppercase">
            Name
          </span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className={cn(
              'mt-1 block w-full rounded-md border border-border bg-elevated/70 px-3 py-2 text-sm',
              'focus:border-border-strong focus:bg-elevated focus:outline-none',
            )}
          />
        </label>
      </div>
    </Section>
  );
}
