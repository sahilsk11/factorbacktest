# Three Small Frontend Fixes — FactorBacktest

## Fix 1: Missing `* 100` on `priceChangeTilNextResampling` Display

**File:** `frontend/src/pages/Backtest/FactorSnapshot.tsx`

**Current behavior (line 282):**
```tsx
<td>{snapshot.assetMetrics[symbol].priceChangeTilNextResampling?.toFixed(2)}%</td>
```

**Issue:** The `priceChangeTilNextResampling` value is stored as a decimal (e.g., `0.05` for 5%), but is displayed directly with `toFixed(2)` and a `%` suffix — showing `5.00%` when the stored value is actually `0.05`. Meanwhile, `assetWeight` on the same table row (line 281) correctly multiplies by 100 before displaying:

```tsx
<td>{(100 * snapshot.assetMetrics[symbol].assetWeight).toFixed(2)}%</td>
```

**Fix:** Multiply `priceChangeTilNextResampling` by 100 to match the pattern used for `assetWeight`:

```tsx
<td>{(snapshot.assetMetrics[symbol].priceChangeTilNextResampling * 100)?.toFixed(2)}%</td>
```

**Risk:** Minimal. Single-line display fix, verified against existing pattern in the same component.

---

## Fix 2: Deposit Input Length Validation Is Too Restrictive

**File:** `frontend/src/pages/Backtest/InvestInStrategy.tsx`

**Current behavior (lines 52–55):**
```tsx
if (!/[^0-9]/.test(x) && x.length < 3) {
  setDepositAmount(parseFloat(x))
}
```

**Issue:** The `x.length < 3` check limits deposits to 2 digits (0–99). A user cannot enter `$100` or higher. The intent appears to be preventing overly large numbers, but the implementation is wrong — `$10` is a valid deposit amount.

**Fix:** Remove the length check entirely, or replace with a sensible upper bound:

```tsx
if (!/[^0-9]/.test(x) && parseFloat(x) <= 1000000) {
  setDepositAmount(parseFloat(x))
}
```

**Risk:** Minimal. The `.` is already blocked by the regex, so decimals aren't an issue here. The upper bound of 1M is generous and prevents accidental overflow.

---

## Fix 3: Hardcoded 500ms Sleep in `setFromUrl`

**File:** `frontend/src/pages/Backtest/Backtest.tsx`

**Current behavior (lines 118–119):**
```tsx
await new Promise(f => setTimeout(f, 500));
setRunBacktestToggle(true);
```

**Issue:** After loading a strategy from URL params, the code sleeps for 500ms before triggering the backtest run. This is a fragile workaround that may fail on slower connections or miss the window on faster ones.

**Fix:** Use proper loading state instead of a fixed timeout:

```tsx
const [loadingStrategy, setLoadingStrategy] = useState(false);
// ...
async function setFromUrl(id: string) {
  const strat = await getStrategy(id)
  if (!strat) {
    setLoadingStrategy(false);
    return
  }
  // ... existing field setting ...
  setLoadingStrategy(true);
  // Let the useEffect that watches loadingStrategy trigger the backtest
}
```

```tsx
useEffect(() => {
  if (loadingStrategy) {
    setRunBacktestToggle(true);
    setLoadingStrategy(false);
  }
}, [loadingStrategy])
```

**Risk:** Low. The current sleep is purely a timing heuristic. Replacing it with explicit state makes behavior deterministic.

---

## Summary

| Fix | File | Lines | Change |
|-----|------|-------|--------|
| 1 | `FactorSnapshot.tsx` | 282 | Add `* 100` to `priceChangeTilNextResampling` |
| 2 | `InvestInStrategy.tsx` | 53–55 | Replace `x.length < 3` with sensible upper bound |
| 3 | `Backtest.tsx` | 118–119 | Replace sleep with proper loading state |

All three are isolated, single-file changes. A reviewer can verify each in under 2 minutes.
