/// <reference types="vite/client" />

// CSS Modules 的类型声明。类名是 camelCase（见 docs/rules/coding-standards.md §4.3）。
declare module '*.module.css' {
  const classes: Readonly<Record<string, string>>
  export default classes
}
