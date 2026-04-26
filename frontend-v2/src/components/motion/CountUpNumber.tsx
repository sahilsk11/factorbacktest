import { animate, useMotionValue, useReducedMotion } from 'framer-motion';
import { useEffect, useState } from 'react';

interface Props {
  // Final value. When this changes we tween from the prior value.
  value: number;
  // Called on each frame to format the displayed string. Letting the
  // caller own formatting keeps the animation logic decoupled from
  // unit conventions (percent, ratio, currency).
  format: (n: number) => string;
  // ms. Default 600ms matches the rest of the entrance choreography.
  durationMs?: number;
}

// Tweens a number from 0 to `value` (and on subsequent updates, from
// the prior value to the new one) and renders the formatted result on
// each frame. Honors prefers-reduced-motion by rendering the final
// value directly with no animation.
//
// Used for headline KPIs (annualized return) on landing-page cards.
// Don't reach for this for every number — overuse looks gimmicky.
export function CountUpNumber({ value, format, durationMs = 600 }: Props): string {
  const reduceMotion = useReducedMotion();
  const motion = useMotionValue(0);
  // Lazy-init: animated path starts at format(0), reduce-motion path
  // is short-circuited at render time below.
  const [display, setDisplay] = useState(() => format(0));

  useEffect(() => {
    if (reduceMotion) return;
    // setState happens inside the rAF callback (onUpdate), not
    // synchronously in the effect body, so the React 19
    // set-state-in-effect rule is satisfied.
    const controls = animate(motion, value, {
      duration: durationMs / 1000,
      ease: 'easeOut',
      onUpdate: (latest) => setDisplay(format(latest)),
    });
    return () => controls.stop();
  }, [value, durationMs, format, motion, reduceMotion]);

  return reduceMotion ? format(value) : display;
}
