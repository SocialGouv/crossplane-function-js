import path from "node:path"
import { fileURLToPath } from "node:url"

import { fixupConfigRules, fixupPluginRules } from "@eslint/compat"
import { FlatCompat } from "@eslint/eslintrc"
import js from "@eslint/js"
import typescriptEslint from "@typescript-eslint/eslint-plugin"
import tsParser from "@typescript-eslint/parser"
import { defineConfig, globalIgnores } from "eslint/config"
import _import from "eslint-plugin-import"
import prettier from "eslint-plugin-prettier"
import globals from "globals"

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const compat = new FlatCompat({
  baseDirectory: __dirname,
  recommendedConfig: js.configs.recommended,
  allConfig: js.configs.all,
})

export default defineConfig([
  // Disable console warnings for logger files
  {
    files: ["**/logger.ts"],
    rules: {
      "no-console": "off",
    },
  },
  globalIgnores([
    "node_modules/**/*",
    "build/**/*",
    "**/build/**/*",
    ".yarn/**/*",
    "tests",
    ".lintstagedrc.js",
    "eslint.config.mjs",
    ".versionrc.cjs",
  ]),
  {
    extends: fixupConfigRules(
      compat.extends(
        "eslint:recommended",
        "plugin:@typescript-eslint/recommended",
        "plugin:import/recommended",
        "plugin:import/typescript",
        "plugin:prettier/recommended"
      )
    ),

    plugins: {
      "@typescript-eslint": fixupPluginRules(typescriptEslint),
      import: fixupPluginRules(_import),
      prettier: fixupPluginRules(prettier),
    },

    languageOptions: {
      globals: {
        ...globals.node,
      },

      parser: tsParser,
      ecmaVersion: "latest",
      sourceType: "module",

      parserOptions: {
        project: ["./tsconfig.json", "./packages/*/tsconfig.json"],
        createDefaultProgram: true,
      },
    },

    settings: {
      "import/resolver": {
        typescript: {
          alwaysTryTypes: true,
          project: ["./tsconfig.json", "./packages/*/tsconfig.json"],
        },
      },
    },

    rules: {
      "@typescript-eslint/explicit-function-return-type": "off",
      "@typescript-eslint/no-explicit-any": "warn",

      "@typescript-eslint/no-unused-vars": [
        "error",
        {
          argsIgnorePattern: "^_",
        },
      ],

      "@typescript-eslint/no-non-null-assertion": "warn",
      "import/no-unresolved": "off",

      "import/order": [
        "error",
        {
          groups: ["builtin", "external", "internal", "parent", "sibling", "index"],
          "newlines-between": "always",

          alphabetize: {
            order: "asc",
            caseInsensitive: true,
          },
        },
      ],

      "no-console": [
        "warn",
        {
          allow: ["warn", "error", "info", "log"],
        },
      ],

      "prettier/prettier": "error",
    },
  },
])
