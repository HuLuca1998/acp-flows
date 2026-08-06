# AGENTS.md · frontend/tests

> **就近优先**。测试策略见 [`../../docs/testing-strategy.md`](../../docs/testing-strategy.md) §6。

## 负责什么

**跨 feature 共享的测试基建**，以及前端测试索引。

```
tests/
├── INDEX.md      ★ 前端全部 Vitest 测试的索引（按文件登记）
├── msw/            由 api/openapi.yaml 生成的 mock handlers
├── fixtures/       录制的事件序列、示例契约/证据数据
└── setup.ts        Vitest 全局 setup
```

## 不负责什么

**不放单个组件的测试。** `EventStream.test.tsx` 与 `EventStream.tsx` 同目录。

理由：测试离被测组件远，改组件时不会有人想起来更新它。

## MSW handler 必须是生成的

```
✗ 手写 mock 响应对象      ← 与 openapi.yaml 漂移，组件测试全绿但联调必炸
✓ make gen 生成           ← spec 改了 mock 自动跟着变
```

## 检查命令

```bash
pnpm -C .. test --run
make -C .. check-test-index
```

## 本域特有的坑

- **写新测试前先查 [`INDEX.md`](INDEX.md)**，按**行为**搜。已覆盖 → 扩展它，不要新开文件。
- **禁止断言 CSS 类名与 DOM 结构。** 用 `getByRole` / `getByText` 按用户看得见的东西查询。
  设计会改，绑死结构的测试只会变成噪音。
- **事件流测试要用 `fixtures/` 里录制的序列**，不要在测试里手写事件对象——
  13 类事件的结构由 openapi 定义，手写的会漂移。
- **Fake `EventSource` 要覆盖乱序与断线**，不只是顺序推送——
  `seq` 归位和 `Last-Event-ID` 续传是产品硬需求。
