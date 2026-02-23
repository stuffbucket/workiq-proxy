import js from "@eslint/js";
import globals from "globals";

export default [
  js.configs.recommended,
  {
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "commonjs",
      globals: {
        ...globals.node,
      },
    },
    rules: {
      "no-unused-vars": ["error", { argsIgnorePattern: "^_" }],
      "no-undef": "error",
      "no-constant-condition": "warn",
      "no-unreachable": "error",
      "eqeqeq": ["error", "always", { null: "ignore" }],
      "no-var": "error",
      "prefer-const": "error",
      "no-throw-literal": "error",
      "no-implicit-globals": "error",
      "consistent-return": "warn",
      "no-shadow": "warn",
      "no-process-exit": "off",
    },
  },
  {
    ignores: ["npm/bin/**", "node_modules/**"],
  },
];
