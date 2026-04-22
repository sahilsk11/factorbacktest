import {
  test as base,
  expect,
  type APIRequestContext,
  type TestType,
  type PlaywrightTestArgs,
  type PlaywrightTestOptions,
  type PlaywrightWorkerArgs,
  type PlaywrightWorkerOptions,
} from '@playwright/test';

export type FixtureName =
  | 'base_universe'
  | 'prices_2020'
  | 'user_basic'
  | 'strategy_momentum'
  | 'investment_basic';

export type FixtureIds = Record<FixtureName, Record<string, string>>;

function baseUrl(): string {
  const override = process.env.TEST_API_URL;
  if (override && override.length > 0) return override;
  const port = process.env.REACT_APP_API_PORT;
  return `http://localhost:${port}`;
}

async function readBody(
  response: { text(): Promise<string> },
): Promise<string> {
  try {
    return await response.text();
  } catch {
    return '<unable to read body>';
  }
}

export async function resetDb(request: APIRequestContext): Promise<void> {
  const url = `${baseUrl()}/__test__/reset`;
  const res = await request.post(url);
  if (!res.ok()) {
    const body = await readBody(res);
    throw new Error(
      `resetDb: POST ${url} failed with status ${res.status()}: ${body}`,
    );
  }
}

function isIdsShape(
  value: unknown,
): value is Record<string, Record<string, string>> {
  if (typeof value !== 'object' || value === null) return false;
  for (const v of Object.values(value as Record<string, unknown>)) {
    if (typeof v !== 'object' || v === null) return false;
    for (const inner of Object.values(v as Record<string, unknown>)) {
      if (typeof inner !== 'string') return false;
    }
  }
  return true;
}

export async function applyFixtures(
  request: APIRequestContext,
  fixtures: FixtureName[],
  opts: { reset?: boolean } = {},
): Promise<Partial<FixtureIds>> {
  const url = `${baseUrl()}/__test__/fixtures`;
  const res = await request.post(url, {
    data: { fixtures, reset: opts.reset ?? false },
  });
  if (!res.ok()) {
    const body = await readBody(res);
    throw new Error(
      `applyFixtures: POST ${url} failed with status ${res.status()}: ${body}`,
    );
  }

  let parsed: unknown;
  try {
    parsed = await res.json();
  } catch (err) {
    const body = await readBody(res);
    throw new Error(
      `applyFixtures: invalid JSON from ${url}: ${
        err instanceof Error ? err.message : String(err)
      }: ${body}`,
    );
  }

  if (
    typeof parsed !== 'object' ||
    parsed === null ||
    !('ids' in parsed)
  ) {
    throw new Error(
      `applyFixtures: unexpected response shape from ${url}: ${JSON.stringify(
        parsed,
      )}`,
    );
  }

  const ids = (parsed as { ids: unknown }).ids;
  if (!isIdsShape(ids)) {
    throw new Error(
      `applyFixtures: unexpected ids shape from ${url}: ${JSON.stringify(ids)}`,
    );
  }

  return ids as Partial<FixtureIds>;
}

export type SeedFn = (names: FixtureName[]) => Promise<Partial<FixtureIds>>;

export const test: TestType<
  PlaywrightTestArgs & PlaywrightTestOptions & { seed: SeedFn },
  PlaywrightWorkerArgs & PlaywrightWorkerOptions
> = base.extend<{ seed: SeedFn }>({
  seed: async ({ request }, use) => {
    await resetDb(request);
    const seed: SeedFn = (names) =>
      applyFixtures(request, names, { reset: false });
    await use(seed);
  },
});

export { expect };
