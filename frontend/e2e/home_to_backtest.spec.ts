import { test, expect } from './fixtures';

test.use({ seedName: 'home_strategies' });

// End-to-end flow:
//  Home.tsx renders published strategies as clickable react-bootstrap cards.
//  Clicking a card navigates to /backtest?id=<uuid>, Backtest.tsx populates
//  form state from /publishedStrategies, Form.tsx POSTs /backtest, and on
//  success the Stats panel ("Performance History") and Inspector ("Holdings
//  History" tab + AssetAllocationTable) render. We don't assume any
//  particular strategy name — only structural signals.
test('landing page -> click strategy -> backtest runs and renders results', async ({
  page,
}) => {
  await page.goto('/');

  await expect(page.locator('h2')).toContainText('Factor Backtest');

  // Each StrategyCard is a react-bootstrap <Card>, which renders the global
  // `.card` class. The landing page has no other .card elements.
  const cards = page.locator('.card');
  await expect(cards.first()).toBeVisible({ timeout: 15_000 });
  expect(await cards.count()).toBeGreaterThanOrEqual(1);

  await cards.first().click();
  await expect(page).toHaveURL(/\/backtest\?id=[0-9a-f-]+/, { timeout: 10_000 });

  // The Stats panel in Backtest.tsx only renders when lastStrategyID is
  // non-null, which is set inside Form.tsx's handleSubmit on successful
  // /backtest response. This is the single strongest signal that the whole
  // chain fired end-to-end. The backtest itself is slow (3y monthly rebalance
  // over SPY_TOP_80 members), so give a generous timeout.
  const perfHistoryHeading = page.getByText('Performance History', { exact: false });
  await expect(perfHistoryHeading).toBeVisible({ timeout: 60_000 });

  // Each of the three stat rows must show a numeric value, not the "n/a"
  // fallback. (The fallback only fires when the metric is falsy; a real
  // backtest on real price data produces non-zero numbers. We assert a digit
  // is present instead of parsing the exact formatting so we stay resilient
  // to "4.32%" vs "0.04" vs future formatting changes.)
  const annReturnRow = page.locator('tr', { hasText: 'Annualized Return' });
  const sharpeRow = page.locator('tr', { hasText: 'Sharpe Ratio' });
  const stdevRow = page.locator('tr', { hasText: /Annualized Volatil.*stdev/i });
  for (const row of [annReturnRow, sharpeRow, stdevRow]) {
    await expect(row.first()).toBeVisible();
    await expect(row.first().locator('td')).toContainText(/[0-9]/);
  }

  // Inspector (FactorSnapshot.tsx) renders only when factorData.length > 0,
  // which also only happens after a successful backtest. The "Holdings
  // History" Nav.Link is the stable anchor.
  await expect(
    page.getByRole('tab', { name: /Holdings History/i }).or(
      page.getByText('Holdings History', { exact: false }),
    ),
  ).toBeVisible();

  // AssetAllocationTable — scoped by its unique "Factor Score" column header
  // to disambiguate from any other table. We expect ≥1 <tbody> row, one per
  // held asset on the selected snapshot date.
  const holdingsTable = page.locator('table', {
    has: page.locator('th', { hasText: 'Factor Score' }),
  });
  await expect(holdingsTable.first()).toBeVisible();
  const holdingRows = holdingsTable.first().locator('tbody tr');
  expect(await holdingRows.count()).toBeGreaterThanOrEqual(1);
});
