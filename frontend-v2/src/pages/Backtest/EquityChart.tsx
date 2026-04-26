import { motion, useMotionValue, useReducedMotion, useSpring } from 'framer-motion';
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';

import type { ChartPoint, ChartSeries } from './chart-types';
import { formatDelta } from '@/lib/format';
import { cn } from '@/lib/utils';

// Visual parameters for the chart. Pulled out so the magic numbers
// have names — the chart is deliberately busy and changing one of
// these without naming it makes diffs unreadable.
const PADDING = { top: 24, right: 80, bottom: 32, left: 16 };
const STRATEGY_COLOR = 'var(--color-gain)';
const STRATEGY_NEGATIVE_COLOR = 'var(--color-loss)';
const BENCHMARK_COLOR = 'var(--color-accent)';
const DRAW_DURATION_S = 1.1;
const FILL_FADE_DURATION_S = 0.6;
const FILL_FADE_DELAY_S = 0.35;

interface Props {
  series: ChartSeries[];
  // The currently-selected/hovered date. Owned by the parent so other
  // panels (DayInspector, MetricsBar's headline) can react to scrubs.
  hoveredDate: string | null;
  onHover: (date: string | null) => void;
  onSelectDate: (date: string) => void;
  selectedDate: string | null;
  // Forces remount → fresh draw-on animation when set changes (e.g.
  // a rerun completes with different data).
  runKey: string;
}

// Top-of-chart hero KPI: the Robinhood-style headline number. Lives in
// the chart so it can flip green/red the instant the strategy line
// crosses zero, matching the line color.
interface HeadlineState {
  pct: number;
  date: string;
  // value in starting-cash units, e.g. $11,234.56. May be undefined
  // for benchmark-only points — we always lock onto the strategy
  // series for the headline so this is effectively always defined.
  value: number | undefined;
}

// ----- Animated big-number readout for the headline. Lives inline so
// the chart owns its choreography end-to-end. -----
function HeadlineNumber({
  pct,
  value,
}: {
  pct: number;
  value: number | undefined;
}): React.ReactNode {
  const reduceMotion = useReducedMotion();
  const motionValue = useMotionValue(pct);
  const spring = useSpring(motionValue, { stiffness: 220, damping: 26, mass: 0.4 });
  const [displayPct, setDisplayPct] = useState(pct);

  useEffect(() => {
    motionValue.set(pct);
  }, [pct, motionValue]);

  useEffect(() => {
    if (reduceMotion) {
      // No spring — render the latest value directly via a setState
      // call inside a microtask, not synchronously in the effect body.
      const id = window.setTimeout(() => setDisplayPct(pct), 0);
      return () => window.clearTimeout(id);
    }
    const unsub = spring.on('change', (v) => setDisplayPct(v));
    return () => unsub();
  }, [spring, reduceMotion, pct]);

  const shown = reduceMotion ? pct : displayPct;
  const colorClass = shown >= 0 ? 'text-gain' : 'text-loss';

  return (
    <div className="pointer-events-none flex flex-col gap-1">
      <span className="text-[11px] uppercase tracking-[0.18em] text-subtle-foreground">
        Strategy return
      </span>
      <div className="flex items-baseline gap-3">
        <span className={cn('font-mono text-5xl font-semibold tracking-tight', colorClass)}>
          {formatDelta(shown)}
        </span>
        {value !== undefined && (
          <span className="font-mono text-base text-muted-foreground">
            ${' '}
            {value.toLocaleString('en-US', {
              minimumFractionDigits: 2,
              maximumFractionDigits: 2,
            })}
          </span>
        )}
      </div>
    </div>
  );
}

export function EquityChart({
  series,
  hoveredDate,
  onHover,
  onSelectDate,
  selectedDate,
  runKey,
}: Props): React.ReactNode {
  const containerRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 0, height: 0 });

  // Resize observer — chart redraws when container width changes.
  useLayoutEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (!entry) return;
      const { width, height } = entry.contentRect;
      setSize({ width, height });
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const strategy = series.find((s) => s.id === 'strategy') ?? null;
  const benchmark = series.find((s) => s.id === 'benchmark') ?? null;

  // Compute the union of all dates so the x-scale covers the full
  // benchmark window even if the strategy snapshots don't quite reach
  // the same end date. Strategy still drives the headline though.
  const { xScale, yScale, allDates } = useMemo(() => {
    const allPoints: ChartPoint[] = [];
    for (const s of series) allPoints.push(...s.points);
    if (allPoints.length === 0 || size.width === 0 || size.height === 0) {
      return {
        xScale: () => 0,
        yScale: () => 0,
        allDates: [],
      };
    }

    const times = allPoints.map((p) => new Date(p.date).getTime());
    const tMin = Math.min(...times);
    const tMax = Math.max(...times);
    const yValues = allPoints.map((p) => p.pctReturn);
    let yMin = Math.min(...yValues, 0);
    let yMax = Math.max(...yValues, 0);
    // 8% padding on top and bottom keeps the line off the edges.
    const yPad = (yMax - yMin) * 0.08 || 0.02;
    yMin -= yPad;
    yMax += yPad;

    const w = size.width - PADDING.left - PADDING.right;
    const h = size.height - PADDING.top - PADDING.bottom;

    const x = (date: string): number => {
      const t = new Date(date).getTime();
      if (tMax === tMin) return PADDING.left;
      return PADDING.left + ((t - tMin) / (tMax - tMin)) * w;
    };
    const y = (v: number): number => {
      if (yMax === yMin) return PADDING.top + h / 2;
      return PADDING.top + ((yMax - v) / (yMax - yMin)) * h;
    };

    const sortedDates = Array.from(new Set(allPoints.map((p) => p.date))).sort();

    return { xScale: x, yScale: y, allDates: sortedDates };
  }, [series, size.width, size.height]);

  // Build SVG path strings for each series. Use a smooth curve via
  // sampled cardinal-style interpolation? Actually keep it monotone and
  // straight — it's more honest for finance and matches Robinhood's
  // tick line.
  const buildPath = useCallback(
    (points: ChartPoint[], close: boolean): string => {
      if (points.length === 0 || size.width === 0) return '';
      const segs: string[] = [];
      points.forEach((p, i) => {
        const x = xScale(p.date);
        const y = yScale(p.pctReturn);
        segs.push(`${i === 0 ? 'M' : 'L'}${x.toFixed(2)},${y.toFixed(2)}`);
      });
      if (close) {
        const last = points[points.length - 1];
        const first = points[0];
        if (last && first) {
          const baselineY = yScale(0);
          segs.push(`L${xScale(last.date).toFixed(2)},${baselineY.toFixed(2)}`);
          segs.push(`L${xScale(first.date).toFixed(2)},${baselineY.toFixed(2)}`);
          segs.push('Z');
        }
      }
      return segs.join(' ');
    },
    [xScale, yScale, size.width],
  );

  // Find the snapshot point (and its index) closest to the hovered
  // date — used for the tooltip + crosshair puck.
  const lockedDate = hoveredDate ?? selectedDate ?? null;
  const lockedStrategyPoint = useMemo<ChartPoint | null>(() => {
    if (!strategy || strategy.points.length === 0) return null;
    if (!lockedDate) return strategy.points[strategy.points.length - 1] ?? null;
    return findClosest(strategy.points, lockedDate);
  }, [strategy, lockedDate]);
  const lockedBenchmarkPoint = useMemo<ChartPoint | null>(() => {
    if (!benchmark || benchmark.points.length === 0 || !lockedDate) return null;
    return findClosest(benchmark.points, lockedDate);
  }, [benchmark, lockedDate]);

  const headline: HeadlineState | null = lockedStrategyPoint
    ? {
        pct: lockedStrategyPoint.pctReturn,
        date: lockedStrategyPoint.date,
        value: lockedStrategyPoint.value,
      }
    : null;

  // Strategy line color follows sign of locked point. This is the
  // single most distinctively-Robinhood touch in the whole chart.
  const strategyColor = headline && headline.pct < 0 ? STRATEGY_NEGATIVE_COLOR : STRATEGY_COLOR;

  // Mouse handlers. Translate clientX → date by linear interpolation
  // over the allDates array; nearest snapshot wins. We don't use
  // d3.bisector because the dataset is ≤ a few thousand points.
  const handleMouse = useCallback(
    (e: React.MouseEvent<SVGSVGElement>): string | null => {
      const svg = e.currentTarget;
      const rect = svg.getBoundingClientRect();
      if (rect.width === 0) return null;
      const localX = e.clientX - rect.left;
      const usableW = size.width - PADDING.left - PADDING.right;
      if (usableW <= 0 || allDates.length === 0) return null;
      const fraction = (localX - PADDING.left) / usableW;
      const clamped = Math.max(0, Math.min(1, fraction));
      const idx = Math.round(clamped * (allDates.length - 1));
      return allDates[idx] ?? null;
    },
    [size.width, allDates],
  );

  const onMouseMove = useCallback(
    (e: React.MouseEvent<SVGSVGElement>) => {
      const d = handleMouse(e);
      if (d) onHover(d);
    },
    [handleMouse, onHover],
  );
  const onMouseLeave = useCallback(() => onHover(null), [onHover]);
  const onClick = useCallback(
    (e: React.MouseEvent<SVGSVGElement>) => {
      const d = handleMouse(e);
      if (d) onSelectDate(d);
    },
    [handleMouse, onSelectDate],
  );

  // ----- Render -----

  const ready = size.width > 0 && size.height > 0 && (strategy?.points.length ?? 0) > 0;

  // Y-axis tick labels — five evenly-spaced percent values across the
  // visible range. Pulled out of the JSX for readability.
  const yTicks = useMemo(() => {
    if (!ready) return [];
    const yValues: number[] = [];
    for (const s of series) for (const p of s.points) yValues.push(p.pctReturn);
    if (yValues.length === 0) return [];
    let yMin = Math.min(...yValues, 0);
    let yMax = Math.max(...yValues, 0);
    const pad = (yMax - yMin) * 0.08 || 0.02;
    yMin -= pad;
    yMax += pad;
    const count = 5;
    const step = (yMax - yMin) / (count - 1);
    return Array.from({ length: count }, (_, i) => yMin + step * i);
  }, [series, ready]);

  return (
    <div ref={containerRef} className="relative h-full w-full overflow-hidden">
      {/* Floating headline (top-left). Lifts above the chart canvas. */}
      <div className="pointer-events-none absolute left-6 top-6 z-10">
        {headline && <HeadlineNumber pct={headline.pct} value={headline.value} />}
        {headline && (
          <div className="mt-1 font-mono text-xs text-muted-foreground">
            {formatDateLong(headline.date)}
          </div>
        )}
      </div>

      {/* Legend (bottom-center). Static — labels never animate. */}
      {benchmark && (
        <div className="pointer-events-none absolute bottom-6 left-1/2 z-10 flex -translate-x-1/2 items-center gap-4 text-xs">
          <LegendDot color={strategyColor} label={strategy?.label ?? 'Strategy'} />
          <LegendDot color={BENCHMARK_COLOR} label={benchmark.label} dashed />
        </div>
      )}

      {ready && (
        <svg
          width="100%"
          height="100%"
          viewBox={`0 0 ${size.width} ${size.height}`}
          className="absolute inset-0 cursor-crosshair"
          onMouseMove={onMouseMove}
          onMouseLeave={onMouseLeave}
          onClick={onClick}
        >
          <defs>
            <linearGradient id={`strategy-fill-${runKey}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={strategyColor} stopOpacity="0.3" />
              <stop offset="100%" stopColor={strategyColor} stopOpacity="0" />
            </linearGradient>
          </defs>

          {/* Y gridlines + labels — sparse, dotted, low-opacity. */}
          {yTicks.map((v, i) => (
            <g key={`tick-${i}`}>
              <line
                x1={PADDING.left}
                x2={size.width - PADDING.right}
                y1={yScale(v)}
                y2={yScale(v)}
                stroke="rgba(255,255,255,0.05)"
                strokeWidth={1}
                strokeDasharray="2 4"
              />
              <text
                x={size.width - PADDING.right + 8}
                y={yScale(v)}
                dy="0.32em"
                fill="var(--color-subtle-foreground)"
                fontSize="11"
                fontFamily="var(--font-mono)"
              >
                {(v * 100).toFixed(1)}%
              </text>
            </g>
          ))}

          {/* Zero baseline emphasized. */}
          <line
            x1={PADDING.left}
            x2={size.width - PADDING.right}
            y1={yScale(0)}
            y2={yScale(0)}
            stroke="rgba(255,255,255,0.18)"
            strokeWidth={1}
          />

          {/* Strategy area fill — fades in slightly behind the stroke. */}
          {strategy && strategy.points.length > 0 && (
            <motion.path
              key={`area-${runKey}`}
              d={buildPath(strategy.points, true)}
              fill={`url(#strategy-fill-${runKey})`}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: FILL_FADE_DELAY_S, duration: FILL_FADE_DURATION_S }}
            />
          )}

          {/* Benchmark line (drawn first so it sits behind strategy). */}
          {benchmark && benchmark.points.length > 0 && (
            <motion.path
              key={`bench-${runKey}`}
              d={buildPath(benchmark.points, false)}
              fill="none"
              stroke={BENCHMARK_COLOR}
              strokeWidth={1.4}
              strokeDasharray="4 4"
              initial={{ pathLength: 0, opacity: 0 }}
              animate={{ pathLength: 1, opacity: 0.7 }}
              transition={{ duration: DRAW_DURATION_S, ease: 'easeOut' }}
            />
          )}

          {/* Strategy line — the hero stroke. */}
          {strategy && strategy.points.length > 0 && (
            <motion.path
              key={`line-${runKey}`}
              d={buildPath(strategy.points, false)}
              fill="none"
              stroke={strategyColor}
              strokeWidth={2.2}
              strokeLinecap="round"
              strokeLinejoin="round"
              initial={{ pathLength: 0 }}
              animate={{ pathLength: 1 }}
              transition={{ duration: DRAW_DURATION_S, ease: 'easeOut' }}
            />
          )}

          {/* Crosshair when hovered or a date is selected. */}
          {lockedDate && lockedStrategyPoint && (
            <Crosshair
              x={xScale(lockedDate)}
              strategyY={yScale(lockedStrategyPoint.pctReturn)}
              benchmarkY={lockedBenchmarkPoint ? yScale(lockedBenchmarkPoint.pctReturn) : null}
              top={PADDING.top}
              bottom={size.height - PADDING.bottom}
              strategyColor={strategyColor}
              benchmarkColor={BENCHMARK_COLOR}
              date={lockedDate}
              isSelected={selectedDate === lockedDate}
            />
          )}

          {/* Glowing endpoint puck — the Robinhood "laser mode" pulse. */}
          {strategy &&
            (() => {
              const last = strategy.points[strategy.points.length - 1];
              if (!last) return null;
              return (
                <PulseDot
                  cx={xScale(last.date)}
                  cy={yScale(last.pctReturn)}
                  color={strategyColor}
                />
              );
            })()}

          {/* Right-edge floating price pill on the strategy line. */}
          {strategy &&
            (() => {
              const last = strategy.points[strategy.points.length - 1];
              if (!last) return null;
              return (
                <RightEdgePill
                  x={size.width - PADDING.right}
                  y={yScale(last.pctReturn)}
                  pct={last.pctReturn}
                />
              );
            })()}
        </svg>
      )}
    </div>
  );
}

// ----- supporting components -----

function Crosshair({
  x,
  strategyY,
  benchmarkY,
  top,
  bottom,
  strategyColor,
  benchmarkColor,
  date,
  isSelected,
}: {
  x: number;
  strategyY: number;
  benchmarkY: number | null;
  top: number;
  bottom: number;
  strategyColor: string;
  benchmarkColor: string;
  date: string;
  isSelected: boolean;
}): React.ReactNode {
  return (
    <g pointerEvents="none">
      <line
        x1={x}
        x2={x}
        y1={top}
        y2={bottom}
        stroke={isSelected ? strategyColor : 'rgba(255,255,255,0.35)'}
        strokeWidth={isSelected ? 1.4 : 1}
        strokeDasharray={isSelected ? undefined : '2 3'}
      />
      <circle
        cx={x}
        cy={strategyY}
        r={5}
        fill="var(--color-background)"
        stroke={strategyColor}
        strokeWidth={2}
      />
      {benchmarkY !== null && (
        <circle
          cx={x}
          cy={benchmarkY}
          r={4}
          fill="var(--color-background)"
          stroke={benchmarkColor}
          strokeWidth={1.5}
        />
      )}
      <text
        x={x}
        y={bottom + 18}
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="11"
        fill="var(--color-muted-foreground)"
      >
        {formatDateShort(date)}
      </text>
    </g>
  );
}

function PulseDot({ cx, cy, color }: { cx: number; cy: number; color: string }): React.ReactNode {
  return (
    <g pointerEvents="none">
      <motion.circle
        cx={cx}
        cy={cy}
        r={6}
        fill={color}
        opacity={0.25}
        initial={{ r: 6, opacity: 0.25 }}
        animate={{ r: 14, opacity: 0 }}
        transition={{ duration: 1.4, repeat: Infinity, ease: 'easeOut' }}
      />
      <circle cx={cx} cy={cy} r={3.5} fill={color} />
    </g>
  );
}

function RightEdgePill({ x, y, pct }: { x: number; y: number; pct: number }): React.ReactNode {
  const text = formatDelta(pct);
  // 64×22 pill with 8px arrow into the line.
  return (
    <g pointerEvents="none" transform={`translate(${x}, ${y})`}>
      <rect
        x={6}
        y={-11}
        width={70}
        height={22}
        rx={4}
        ry={4}
        fill={pct >= 0 ? 'var(--color-gain)' : 'var(--color-loss)'}
        opacity={0.95}
      />
      <text
        x={41}
        y={0}
        dy="0.32em"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="12"
        fontWeight={600}
        fill="#0a0a0b"
      >
        {text}
      </text>
    </g>
  );
}

function LegendDot({
  color,
  label,
  dashed,
}: {
  color: string;
  label: string;
  dashed?: boolean;
}): React.ReactNode {
  return (
    <span className="inline-flex items-center gap-2 text-muted-foreground">
      <span
        aria-hidden
        className={cn('inline-block h-0.5 w-5', dashed && 'border-t border-dashed')}
        style={{ backgroundColor: dashed ? 'transparent' : color, borderColor: color }}
      />
      <span className="font-mono">{label}</span>
    </span>
  );
}

// ----- date helpers (chart-local) -----

function findClosest(points: ChartPoint[], target: string): ChartPoint | null {
  const head = points[0];
  if (!head) return null;
  const targetT = new Date(target).getTime();
  let best = head;
  let bestDiff = Math.abs(new Date(best.date).getTime() - targetT);
  for (const p of points) {
    const diff = Math.abs(new Date(p.date).getTime() - targetT);
    if (diff < bestDiff) {
      best = p;
      bestDiff = diff;
    }
  }
  return best;
}

function formatDateLong(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' });
}

function formatDateShort(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: '2-digit' });
}
