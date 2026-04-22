import { execFileSync } from 'child_process';
import path from 'path';

export default async function globalSetup() {
  const repoRoot = path.resolve(__dirname, '../..');
  const binOut = process.env.FB_TEST_API_BIN ?? '/tmp/fb-test-api';
  process.env.FB_TEST_API_BIN = binOut;
  execFileSync(
    'go',
    ['build', '-o', binOut, './cmd/test-api'],
    { cwd: repoRoot, stdio: 'inherit' },
  );
}
