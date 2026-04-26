import js from '@eslint/js';
import globals from 'globals';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import importX from 'eslint-plugin-import-x';
import { createTypeScriptImportResolver } from 'eslint-import-resolver-typescript';
import unusedImports from 'eslint-plugin-unused-imports';
import prettier from 'eslint-config-prettier';
import tseslint from 'typescript-eslint';
import { defineConfig, globalIgnores } from 'eslint/config';

// Single source of truth for FE lint rules. The high-level posture:
//
//   - We let the AI write the code, so the rules are tuned to catch
//     classes of mistakes that get past humans (floating promises,
//     misused promises, cyclic imports, dead handlers) rather than
//     style. Style is owned by Prettier.
//   - `npm run lint` runs with `--max-warnings 0`, so anything below
//     blocks CI. There is no "warn that nobody reads" tier — if a rule
//     isn't worth blocking on, it isn't on.
//   - One structural rule: `max-lines: 700`. Combined with
//     `import-x/no-cycle` it physically prevents the "1000-line god
//     component" pattern the legacy frontend grew into.
export default defineConfig([
  globalIgnores(['dist', 'node_modules']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.strict,
      tseslint.configs.stylistic,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
      // Must be last: turns off any stylistic rules that would fight
      // Prettier. Don't add rules below that re-enable formatting
      // concerns — keep formatting in Prettier.
      prettier,
    ],
    plugins: {
      'import-x': importX,
      'unused-imports': unusedImports,
    },
    languageOptions: {
      globals: globals.browser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    settings: {
      'import-x/resolver-next': [
        createTypeScriptImportResolver({
          project: ['./tsconfig.app.json', './tsconfig.node.json'],
          // Project references emit a benign warning on every run otherwise.
          noWarnOnMultipleProjects: true,
        }),
      ],
    },
    rules: {
      // Hard structural cap — the legacy frontend's 1028-line Form.tsx
      // is the bug we're preventing here.
      'max-lines': ['error', { max: 700, skipBlankLines: true, skipComments: true }],

      // Bug-class rules. These catch real mistakes the AI (and humans)
      // make often.
      '@typescript-eslint/no-explicit-any': 'error',
      '@typescript-eslint/no-floating-promises': 'error',
      '@typescript-eslint/no-misused-promises': 'error',
      '@typescript-eslint/consistent-type-imports': 'error',
      '@typescript-eslint/switch-exhaustiveness-check': 'error',
      'react-hooks/exhaustive-deps': 'error',
      'import-x/no-cycle': ['error', { maxDepth: 10 }],
      'import-x/no-duplicates': 'error',
      'import-x/order': [
        'error',
        {
          groups: [
            ['builtin', 'external'],
            ['internal', 'parent', 'sibling', 'index'],
          ],
          'newlines-between': 'always',
          alphabetize: { order: 'asc', caseInsensitive: true },
        },
      ],

      // Replaces `@typescript-eslint/no-unused-vars` with the
      // unused-imports plugin so `--fix` can auto-strip dead imports
      // (TS-ESLint's version doesn't auto-fix imports).
      '@typescript-eslint/no-unused-vars': 'off',
      'unused-imports/no-unused-imports': 'error',
      'unused-imports/no-unused-vars': [
        'error',
        { vars: 'all', varsIgnorePattern: '^_', args: 'after-used', argsIgnorePattern: '^_' },
      ],

      // Direct fixes for legacy-frontend habits we're not bringing
      // forward: `alert(err.message)` for errors, ad-hoc `console.log`
      // debug statements, `==` comparisons.
      'no-alert': 'error',
      'no-console': ['warn', { allow: ['warn', 'error'] }],
      eqeqeq: ['error', 'always'],
      'prefer-const': 'error',
    },
  },
]);
