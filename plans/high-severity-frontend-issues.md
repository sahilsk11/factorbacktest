# High-Severity Frontend Issues — FactorBacktest

## 1. Dual Auth System Causing Identity Confusion

**File:** `frontend/src/pages/Backtest/InvestInStrategy.tsx` (lines 76–93)  
**File:** `frontend/src/pages/Backtest/Form.tsx` (lines 33–34)

Two completely different auth mechanisms fire simultaneously:
- `useGoogleLogin` from `@react-oauth/google` sets a `GoogleAuthUser.accessToken` cookie
- `useAuth()` from `auth.tsx` returns a Supabase `Session` with its own separate access token

API calls in the same component use different tokens depending on which auth hook was used. Token expiry on either path causes silent failures or unhandled errors.

---

## 2. No In-Flight Request Guard on `handleSubmit`

**File:** `frontend/src/pages/Backtest/Form.tsx` (lines 145–149)

```tsx
useEffect(() => {
  if (runBacktestToggle) {
    handleSubmit(null)
  }
}, [runBacktestToggle])
```

If `runBacktestToggle` toggles twice in quick succession, `handleSubmit` fires twice. No guard (e.g., `if (loading) return`) prevents concurrent API calls.

---

## 3. `getIsBookmarked` Fires on Every Keystroke

**File:** `frontend/src/pages/Backtest/Form.tsx` (lines 375–388)

```tsx
useEffect(() => {
  getIsBookmarked(session, props).then(...)
}, [session, props])
```

`props` includes `factorExpression`, `factorName`, and other form fields — every character typed triggers an API call. No debounce or throttle.

---

## 4. `document.getElementById` in Render Body

**File:** `frontend/src/pages/Backtest/Form.tsx` (lines 319–332)

```tsx
const cashInput = document.getElementById("cash");
if (cash <= 0) {
  (cashInput as HTMLInputElement)?.setCustomValidity(...)
}
```

DOM side-effect during render. Breaks React StrictMode, returns null if element hasn't mounted, and is a textbook React anti-pattern.

---

## 5. No Error Boundaries

**File:** All components

Any thrown error (parse failure, undefined access, bad API response shape) crashes the entire app to a white screen. No graceful degradation or retry UI.

---

## 6. Null/Undefined Display Risk in Price Change Column

**File:** `frontend/src/pages/Backtest/FactorSnapshot.tsx` (line 282)

```tsx
<td>{(snapshot.assetMetrics[symbol].priceChangeTilNextResampling * 100)?.toFixed(2)}%</td>
```

If `priceChangeTilNextResampling` is `undefined` (not `null`), `(undefined * 100)?.toFixed(2)` returns `"NaN%"`.

---

## 7. Division by Zero in `InvestmentTile`

**File:** `frontend/src/pages/Investments/Invest.tsx` (lines 96–98)

```tsx
weights[h.symbol] = h.marketValue / stats.currentValue
```

No guard for `stats.currentValue === 0`. Will produce `Infinity` or `NaN`.

---

## 8. Type Model Doesn't Reflect Runtime Reality

**File:** `frontend/src/models.ts`

- `GetPublishedStrategiesResponse.createdAt` typed as `Date` but backend sends ISO strings
- `BacktestInputs.numAssets` is required; `BacktestRequest.numAssets` is optional

These mismatches cause silent runtime failures when fields are missing or wrong types are deserialized.

---

## 9. Zero Test Coverage

**File:** `frontend/src/`

No Vitest tests, no React Testing Library. Every refactor is a dice roll.

---

## 10. Form Props Drilled 6+ Levels

**File:** `frontend/src/pages/Backtest/Form.tsx`

`FormViewProps` is passed through `ClassicFormView` → `BookmarkStrategy` → child components. No context, no state management. Adding a single form field requires touching 4+ files.

---

**Priority:** Items 1–3 are the most likely to cause real user-facing failures. Items 5 and 9 are systemic risks.
