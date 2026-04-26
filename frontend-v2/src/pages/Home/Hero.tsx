import { useNavigate } from 'react-router';

import { Button } from '@/components/ui/button';

// Hero section above the strategy grid. Title, subtitle, primary CTA
// to the backtest builder. No marketing fluff — the product is the
// chart.
export function Hero(): React.ReactNode {
  const navigate = useNavigate();
  return (
    <section className="mx-auto max-w-7xl px-6 pt-16 pb-12">
      <div className="flex flex-col items-start gap-6">
        <div>
          <h1 className="text-4xl font-semibold tracking-tight text-foreground sm:text-5xl">
            Factor Backtest
          </h1>
          <p className="mt-3 max-w-xl text-base text-muted-foreground">
            Create and backtest factor-based investment strategies. Compose factors, simulate
            against historical asset universes, and evaluate risk-adjusted performance.
          </p>
        </div>
        <Button
          size="lg"
          onClick={() => {
            // navigate returns a Promise in react-router v7; void it so
            // ESLint's no-misused-promises stays happy and so an
            // accidental navigation rejection doesn't surface unhandled.
            void navigate('/builder');
          }}
        >
          Create Strategy
        </Button>
      </div>
    </section>
  );
}
