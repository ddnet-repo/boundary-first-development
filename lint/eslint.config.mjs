// Boundary-First Development — the TypeScript/JavaScript gate (BFD-17).
// https://codeberg.org/galaxi/boundary-first-development — RULES.md is the law;
// this file is that law in ESLint's dialect. Copy it to your project root as
// eslint.config.mjs (needs: npm i -D eslint typescript-eslint). Extend it
// freely — these rules are the floor, not a ceiling.
import { defineConfig } from 'eslint/config';
import tseslint from 'typescript-eslint';

export default defineConfig(
  {
    files: ['**/*.{js,mjs,cjs,ts,mts,cts}'],
    extends: [tseslint.configs.recommended],
    rules: {
      // BFD-16: `any` is a hole in the contract
      '@typescript-eslint/no-explicit-any': 'error',

      // BFD-15: functions accept a single struct — one argument, named fields
      'max-params': ['error', { max: 1 }],

      // BFD-29: nothing ships provisionally — no markers, no swallowed
      // failures, no suppressions standing in for work not done
      'no-warning-comments': [
        'error',
        { terms: ['todo', 'fixme', 'hack', 'xxx'], location: 'anywhere' },
      ],
      'no-empty': ['error', { allowEmptyCatch: false }],
      '@typescript-eslint/ban-ts-comment': [
        'error',
        // A described suppression is still a suppression (BFD-27).
        { 'ts-expect-error': true, 'ts-ignore': true, 'ts-nocheck': true, 'ts-check': false },
      ],

      // BFD-11: camelCase on the frontend; translation happens at the boundary
      '@typescript-eslint/naming-convention': [
        'error',
        { selector: 'default', format: ['camelCase'] },
        { selector: 'variable', format: ['camelCase', 'UPPER_CASE', 'PascalCase'] },
        { selector: 'parameter', format: ['camelCase'], leadingUnderscore: 'allow' },
        { selector: 'typeLike', format: ['PascalCase'] },
        { selector: 'enumMember', format: ['camelCase'] },
        { selector: 'import', format: null },
        {
          selector: ['objectLiteralProperty', 'typeProperty'],
          modifiers: ['requiresQuotes'],
          format: null,
        },
      ],

      // BFD-22: components never make API calls; all HTTP lives in the API service
      'no-restricted-globals': [
        'error',
        { name: 'fetch', message: 'BFD-22: all backend communication routes through the API service.' },
        { name: 'XMLHttpRequest', message: 'BFD-22: all backend communication routes through the API service.' },
        { name: 'EventSource', message: 'BFD-22: all backend communication routes through the API service.' },
        { name: 'WebSocket', message: 'BFD-22: all backend communication routes through the API service.' },
      ],
      'no-restricted-imports': [
        'error',
        {
          paths: ['axios', 'ky', 'got', 'superagent', 'node-fetch'].map((name) => ({
            name,
            message: 'BFD-22: HTTP clients live in the API service, nowhere else.',
          })),
        },
      ],
    },
  },
  {
    // The one place allowed to speak HTTP. Point the glob at your API service.
    files: ['**/services/api/**'],
    rules: {
      'no-restricted-globals': 'off',
      'no-restricted-imports': 'off',
    },
  },
);
