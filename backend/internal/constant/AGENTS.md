# AGENTS.md · backend/internal/constant

> **就近优先**。规则见 [`../../../docs/rules/coding-standards.md`](../../../docs/rules/coding-standards.md) §1.2。

## 负责什么

**跨包共享**的常量、枚举取值、固定配置。按主题分文件，一个主题一个文件。

```
constant/
├── state.go      Work / Unit / Attempt 等状态机取值
├── event.go      13 类事件 type
├── decision.go   D0–D3 等级
├── path.go       .acpflows 下的固定路径片段
├── acp.go        ACP 方法名
└── limit.go      超时、重试次数、体积上限
```

## 不负责什么

- **不放只在一个文件里用到的常量** —— 就地定义，别污染共享命名空间
- **不放配置** —— 会变的东西不是常量，走环境变量或 DB
- **不放业务逻辑** —— 枚举的 `IsValid()` / `String()` 可以，判断「能不能从 A 到 B」不行（那是 `domain`）

## 依赖方向

| | |
|---|---|
| 允许 import | **仅标准库** |
| 禁止 import | 本仓库任何其他包、任何第三方库 |

这一层比 `domain` 还底层——`domain` import 它，它不 import 任何人。

## 枚举的四件套

每个枚举必须齐备，缺一项都会在后面付代价：

1. **具名类型**，不用裸 `string` / `int`
2. **常量组**，每个取值一行注释说明语义
3. **`AllXxx()`** 返回全部合法取值的**副本**（不是内部切片）
4. **`IsValid()` + `String()`**

再加一条：**必须有穷举测试**。新增取值而忘了在别处处理时，测试要红。

```go
type WorkState string
const WorkStateExecuting WorkState = "executing"
func AllWorkStates() []WorkState { … }   // 返回副本
func (s WorkState) IsValid() bool { … }
func (s WorkState) String() string { … }
```

## 检查命令

```bash
cd backend && go test ./internal/constant/... -count=1
```

## 本域特有的坑

- **改这里的枚举取值 = 改协议。** 状态词会出现在 API 响应、事件流、
  界面上（等宽显示、不翻译）。改一个值要同步改：`AGENTS.md` §8 术语表 +
  `api/openapi.yaml` + 前端 `constants/` + 数据库里的历史数据
- **`AllXxx()` 返回内部切片会被调用方改坏。** 一律 `copy` 出副本
- **枚举值存数据库时存字符串原值**，不存整数（见 `docs/rules/database.md` §2）
