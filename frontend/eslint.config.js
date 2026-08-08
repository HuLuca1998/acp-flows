import js from '@eslint/js'
import tseslint from 'typescript-eslint'
import reactHooks from 'eslint-plugin-react-hooks'
import importPlugin from 'eslint-plugin-import'
import unicorn from 'eslint-plugin-unicorn'
import globals from 'globals'

/**
 * 设计合规与工程规范。规则来源：
 *   - design/_ds/nocturne/_adherence.oxlintrc.json 的三条（禁裸 hex / 禁裸 px / 字体只能 Inter）
 *   - docs/coding-standards.md §4
 *   - docs/frontend-guide.md §13
 *   - docs/i18n.md §6
 *
 * 加规则时同步更新上述文档——规则与文档对不上时以文档为准，说明规则写错了。
 */
export default tseslint.config(
  {
    ignores: [
      'dist',
      'src/api/gen',
      'node_modules',
      'coverage',
      // 设计稿的本地对照副本（design/PARITY.md 说明怎么用）——
      // 它是别人的产物，不该受我们的 lint 规则约束
      'public/design',
    ],
  },

  js.configs.recommended,
  ...tseslint.configs.recommendedTypeChecked,

  // 配置文件与纯 JS 不参与类型化 lint —— 它们不在 tsconfig 的 project 里。
  // ★ 必须放在 recommendedTypeChecked 之后，否则会被它覆盖回去。
  {
    files: ['**/*.js', '**/*.mjs'],
    ...tseslint.configs.disableTypeChecked,
  },

  {
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      globals: { ...globals.browser },
      parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname },
    },
    plugins: { 'react-hooks': reactHooks, import: importPlugin, unicorn },
    settings: { 'import/resolver': { typescript: true } },
    rules: {
      ...reactHooks.configs.recommended.rules,

      // ── 设计系统合规（转译自 _adherence.oxlintrc.json）──────────
      'no-restricted-syntax': [
        'error',
        {
          selector: 'Literal[value=/#[0-9a-fA-F]{3,8}\\b/]',
          message: '禁止写死 hex —— 用设计令牌 var(--color-*)。见 docs/frontend-guide.md §1',
        },
        {
          selector: 'Literal[value=/^\\d+px$/]',
          message: '禁止裸 px —— 用 var(--space-*) / var(--radius-*) / var(--layout-*)',
        },
        {
          // 界面文案必须走 t()：JSX 里出现中日韩字符即报错
          selector: 'JSXText[value=/[\\u4e00-\\u9fa5]/]',
          message: '界面文案必须走 t(\'key\')，见 docs/i18n.md',
        },
        {
          selector: 'TSEnumDeclaration',
          message: '禁用 enum —— 用 as const 对象 + 联合类型，见 docs/coding-standards.md §4.1',
        },
        {
          selector: 'SwitchStatement > MemberExpression[property.name="type"]',
          message: '同类实现用注册表分发，不用 switch。见 docs/design-principles.md §2.5',
        },
      ],

      // ── 分层：只有 platform/ 能碰 Tauri ─────────────────────────
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: ['@tauri-apps/*'],
              message: '只有 src/platform/ 可以 import @tauri-apps/* —— 否则 Web 版当天就废。见 docs/architecture.md §5',
            },
            {
              group: ['../../../*'],
              message: '跨三层以上的相对路径请改用 @/ 别名',
            },
          ],
        },
      ],

      // ── 命名与组织 ─────────────────────────────────────────────
      'unicorn/filename-case': [
        'error',
        {
          cases: { kebabCase: true, pascalCase: true },
          ignore: [/^vite\.config\.ts$/, /^eslint\.config\.js$/],
        },
      ],
      'import/order': [
        'error',
        {
          groups: ['builtin', 'external', 'internal', 'parent', 'sibling', 'index'],
          pathGroups: [{ pattern: '@/**', group: 'internal' }],
          'newlines-between': 'always',
          alphabetize: { order: 'asc' },
        },
      ],
      '@typescript-eslint/consistent-type-imports': 'error',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],

      // ── 组件里禁止直接 fetch，一律走生成的 client ───────────────
      'no-restricted-globals': [
        'error',
        { name: 'fetch', message: '走 src/api/ 的生成客户端 + TanStack Query，不要直接 fetch' },
      ],
    },
  },

  // ★ src/api/ 是唯一允许裸 fetch 的地方。
  //
  // 上面那条规则的本意是「网络调用集中在 api 层」，而不是「永远不许 fetch」——
  // SSE 走不了 openapi-fetch（它处理的是一次性 JSON 响应，不吐流），
  // 而 EventSource 带不了 Authorization 头（真机上 /v1/events 一路 401）。
  // 具体理由写在 src/api/events.ts 的文件头。
  {
    files: ['src/api/**/*.ts'],
    rules: { 'no-restricted-globals': 'off' },
  },

  // platform/ 是唯一可以 import Tauri 的地方
  {
    files: ['src/platform/**/*.{ts,tsx}'],
    rules: { 'no-restricted-imports': 'off' },
  },

  // 测试文件放宽：可以用中文断言文案、可以直接 fetch mock
  {
    files: ['**/*.test.{ts,tsx}', 'tests/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-syntax': 'off',
      'no-restricted-globals': 'off',
      '@typescript-eslint/no-unsafe-assignment': 'off',
    },
  },
)
