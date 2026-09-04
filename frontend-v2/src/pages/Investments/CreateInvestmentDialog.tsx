import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { LoaderCircle, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

import {
  createInvestment,
  DEFAULT_CREATE_INVESTMENT_FORM,
  validateCreateInvestmentForm,
  type CreateInvestmentForm,
} from './create-investment';
import { investmentsQueryKey } from './investments-query';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { apiClient, isApiError } from '@/lib/api';
import { cn } from '@/lib/utils';
import { FACTOR_PRESETS } from '@/pages/Builder/presets';
import type { AssetUniverse, RebalanceInterval } from '@/pages/Builder/types';

const REBALANCE_OPTIONS: { value: RebalanceInterval; label: string }[] = [
  { value: 'daily', label: 'Daily' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'monthly', label: 'Monthly' },
];

export function CreateInvestmentDialog({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}): React.ReactNode {
  const queryClient = useQueryClient();
  const dialogRef = useRef<HTMLElement>(null);
  const [form, setForm] = useState<CreateInvestmentForm>(DEFAULT_CREATE_INVESTMENT_FORM);
  const [validationError, setValidationError] = useState<string | null>(null);

  const { data: universes, isLoading: universesLoading } = useQuery({
    queryKey: ['assetUniverses'],
    queryFn: () => apiClient.get<AssetUniverse[]>('/assetUniverses'),
    enabled: open,
  });

  const universeSize =
    universes?.find((u) => u.code === form.assetUniverse)?.numAssets ??
    universes?.[0]?.numAssets ??
    0;
  const maxAssets = Math.max(3, universeSize || 500);

  useEffect(() => {
    if (!open) return;

    const previousOverflow = document.body.style.overflow;
    dialogRef.current?.focus();

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };

    document.body.style.overflow = 'hidden';
    window.addEventListener('keydown', onKeyDown);
    return () => {
      window.removeEventListener('keydown', onKeyDown);
      document.body.style.overflow = previousOverflow;
    };
  }, [onClose, open]);

  const submit = useMutation({
    mutationFn: () => createInvestment(form),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: investmentsQueryKey });
      onClose();
    },
  });

  if (!open) return null;

  const submitError = submit.error
    ? isApiError(submit.error)
      ? submit.error.message
      : submit.error instanceof Error
        ? submit.error.message
        : 'Failed to create investment'
    : null;

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    const error = validateCreateInvestmentForm(form);
    setValidationError(error);
    if (error) return;
    submit.mutate();
  };

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 px-4 py-8 backdrop-blur-sm"
      role="presentation"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="create-investment-title"
        tabIndex={-1}
        className="relative flex max-h-[min(90vh,900px)] w-full max-w-2xl flex-col overflow-hidden rounded-lg border border-border bg-card shadow-xl"
      >
        <div className="flex items-start justify-between border-b border-border px-5 py-4">
          <div>
            <h2 id="create-investment-title" className="text-lg font-semibold text-foreground">
              Create new investment
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Pick a strategy and dollar amount to start paper trading.
            </p>
          </div>
          <button
            type="button"
            className="inline-flex size-8 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-elevated hover:text-foreground"
            aria-label="Close"
            onClick={onClose}
          >
            <X className="size-4" aria-hidden />
          </button>
        </div>

        <form onSubmit={handleSubmit} noValidate className="flex min-h-0 flex-1 flex-col">
          <div className="space-y-5 overflow-y-auto px-5 py-5">
            <label className="block" htmlFor="create-investment-name">
              <span className="text-xs font-medium uppercase tracking-widest text-subtle-foreground">
                Strategy name
              </span>
              <Input
                id="create-investment-name"
                className="mt-1"
                value={form.strategyName}
                onChange={(event) => {
                  setForm((prev) => ({ ...prev, strategyName: event.target.value }));
                  setValidationError(null);
                }}
              />
            </label>

            <label className="block" htmlFor="create-investment-amount">
              <span className="text-xs font-medium uppercase tracking-widest text-subtle-foreground">
                Investment amount (USD)
              </span>
              <Input
                id="create-investment-amount"
                className="mt-1"
                inputMode="numeric"
                min={1}
                type="number"
                value={form.amountDollars}
                onChange={(event) => {
                  const next = Number(event.target.value);
                  setForm((prev) => ({
                    ...prev,
                    amountDollars: Number.isFinite(next) ? next : 0,
                  }));
                  setValidationError(null);
                }}
              />
            </label>

            <div>
              <span className="text-xs font-medium uppercase tracking-widest text-subtle-foreground">
                Factor expression
              </span>
              <div className="mt-2 flex flex-wrap gap-2">
                {FACTOR_PRESETS.map((preset) => (
                  <button
                    key={preset.id}
                    type="button"
                    onClick={() => {
                      setForm((prev) => ({
                        ...prev,
                        factorExpression: preset.expression,
                        strategyName: prev.strategyName || preset.name,
                      }));
                      setValidationError(null);
                    }}
                    className="rounded-md border border-border bg-elevated/50 px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:border-border-strong hover:text-foreground"
                  >
                    {preset.name}
                  </button>
                ))}
              </div>
              <textarea
                value={form.factorExpression}
                onChange={(event) => {
                  setForm((prev) => ({ ...prev, factorExpression: event.target.value }));
                  setValidationError(null);
                }}
                rows={2}
                spellCheck={false}
                className={cn(
                  'mt-3 block w-full resize-none rounded-md border border-border bg-elevated/70 px-3 py-2',
                  'font-mono text-sm text-foreground focus:border-border-strong focus:bg-elevated focus:outline-none',
                )}
              />
            </div>

            <div>
              <span className="text-xs font-medium uppercase tracking-widest text-subtle-foreground">
                Asset universe
              </span>
              {universesLoading ? (
                <div className="mt-2 h-20 animate-pulse rounded-md border border-border bg-elevated/40" />
              ) : (
                <div className="mt-2 grid grid-cols-1 gap-2 sm:grid-cols-2">
                  {(universes ?? [])
                    .filter((u) => u.code.startsWith('SPY_TOP_'))
                    .map((universe) => {
                      const active = universe.code === form.assetUniverse;
                      return (
                        <button
                          key={universe.code}
                          type="button"
                          onClick={() => {
                            setForm((prev) => ({ ...prev, assetUniverse: universe.code }));
                            setValidationError(null);
                          }}
                          className={cn(
                            'rounded-md border px-3 py-2 text-left text-sm transition-colors',
                            active
                              ? 'border-accent bg-accent/10 text-foreground'
                              : 'border-border bg-elevated/50 text-muted-foreground hover:border-border-strong',
                          )}
                        >
                          <span className="font-medium">{universe.displayName}</span>
                          <span className="mt-0.5 block font-mono text-xs text-muted-foreground">
                            {universe.numAssets} assets
                          </span>
                        </button>
                      );
                    })}
                </div>
              )}
            </div>

            <div>
              <span className="text-xs font-medium uppercase tracking-widest text-subtle-foreground">
                Rebalance interval
              </span>
              <div className="mt-2 grid grid-cols-3 gap-2">
                {REBALANCE_OPTIONS.map((option) => {
                  const active = option.value === form.rebalanceInterval;
                  return (
                    <button
                      key={option.value}
                      type="button"
                      onClick={() => {
                        setForm((prev) => ({ ...prev, rebalanceInterval: option.value }));
                        setValidationError(null);
                      }}
                      className={cn(
                        'rounded-md border px-3 py-2 text-sm font-medium transition-colors',
                        active
                          ? 'border-accent bg-accent/15 text-foreground'
                          : 'border-border bg-elevated/50 text-muted-foreground hover:border-border-strong',
                      )}
                    >
                      {option.label}
                    </button>
                  );
                })}
              </div>
            </div>

            <div>
              <span className="text-xs font-medium uppercase tracking-widest text-subtle-foreground">
                Number of assets
              </span>
              <div className="mt-2 flex items-center gap-4">
                <input
                  type="range"
                  min={3}
                  max={maxAssets}
                  step={1}
                  value={Math.min(form.numAssets, maxAssets)}
                  onChange={(event) => {
                    setForm((prev) => ({
                      ...prev,
                      numAssets: parseInt(event.target.value, 10),
                    }));
                    setValidationError(null);
                  }}
                  style={{ accentColor: 'var(--color-accent)' }}
                  className="h-2 flex-1 cursor-pointer"
                />
                <input
                  type="number"
                  min={3}
                  max={maxAssets}
                  value={Math.min(form.numAssets, maxAssets)}
                  onChange={(event) => {
                    const next = parseInt(event.target.value, 10);
                    if (Number.isFinite(next)) {
                      setForm((prev) => ({ ...prev, numAssets: next }));
                      setValidationError(null);
                    }
                  }}
                  className="h-10 w-20 rounded-md border border-border bg-elevated/70 px-3 text-center font-mono text-sm focus:border-border-strong focus:outline-none"
                />
              </div>
            </div>
          </div>

          <div className="border-t border-border px-5 py-4">
            {(validationError || submitError) && (
              <p className="mb-3 text-sm text-loss">{validationError ?? submitError}</p>
            )}
            <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <Button type="button" variant="outline" disabled={submit.isPending} onClick={onClose}>
                Cancel
              </Button>
              <Button type="submit" disabled={submit.isPending}>
                {submit.isPending ? (
                  <>
                    <LoaderCircle className="size-4 animate-spin" aria-hidden />
                    Creating...
                  </>
                ) : (
                  'Create investment'
                )}
              </Button>
            </div>
          </div>
        </form>
      </section>
    </div>,
    document.body,
  );
}
