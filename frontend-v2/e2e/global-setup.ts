import { execFileSync } from 'child_process';
import path from 'path';
import { fileURLToPath } from 'url';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');

export default async function globalSetup() {
  const binOut = process.env.FB_TEST_API_BIN ?? '/tmp/fb-test-api';
  process.env.FB_TEST_API_BIN = binOut;
  execFileSync('go', ['build', '-o', binOut, './cmd/test-api'], {
    cwd: repoRoot,
    stdio: 'inherit',
  });
}
