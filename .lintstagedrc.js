export default {
  '**/*.{js,ts}': [
    'eslint --fix',
  ],
  '**/*.ts': () => 'tsc -p tsconfig.json --noEmit',
  '**/*.{json,md}': [
    'prettier --write',
  ],
}