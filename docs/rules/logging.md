# 日志规范

> 日志是 **AI 调试时的唯一观测面**。人可以 attach 调试器、看界面、凭经验猜；
> AI 只能看日志。所以这里的标准比一般项目高：**结构化、可查询、可按域调级别**。

---

## 1. 两个去处，各有分工

| 去处 | 给谁看 | 内容 |
|---|---|---|
| **stderr** | 人 / `make dev-logs` | 文本格式，只有 `INFO` 以上 |
| **SQLite `logs` 表** | **AI**（用 `db-operate` skill 查） | 结构化，全部级别，带关联字段 |

**落库是重点。** `tail -f` 只能看时间序，AI 真正需要的是
「这个 Work 的所有 ERROR」「这次 attempt 里 acp 组件说了什么」——那是查询，不是滚动。

```sql
-- AI 排查时的典型查询
SELECT ts, level, component, msg FROM logs
WHERE work_id = 'work-08' AND level >= 3 ORDER BY ts DESC LIMIT 50;
```

---

## 2. 五个级别

| 级别 | slog 值 | 什么时候用 | 落库 | 到 stderr |
|---|---|---|---|---|
| `TRACE` | -8 | 协议报文全文、SQL 语句、逐帧事件 | ✅ | ❌ |
| `DEBUG` | -4 | 状态迁移、分支判断、缓存命中 | ✅ | 仅 `-level=debug` |
| `INFO` | 0 | 生命周期事件：服务启动、会话建立、单元验收 | ✅ | ✅ |
| `WARN` | 4 | 降级发生了、重试了、探针没过 | ✅ | ✅ |
| `ERROR` | 8 | 操作失败且未恢复 | ✅ | ✅ |

### 判定标准（避免级别通胀）

- **`ERROR` 必须是"有人要去处理"的。** 用户取消导致的失败不是 ERROR，是 INFO
- **`WARN` 是"能继续但不该发生"。** 降级、重试、探针失败
- **`INFO` 一行 = 一个业务事实。** 不要在循环里打 INFO
- **`DEBUG` 是给排查用的，可以密。** 它不进 stderr，不影响可读性
- **`TRACE` 是全文转储。** ACP 报文、SQL、事件帧——量很大，默认不开

> 级别通胀的典型症状：满屏 ERROR 但系统正常运行。
> 一旦发生，ERROR 就失去了「要去处理」的含义。

---

## 3. 按域调级别

一次调试通常只关心一个组件。全局调到 DEBUG 会淹没在噪音里。

```bash
make dev LOG=debug                 # 全局 debug
make dev LOG=acp=trace             # 只有 acp 组件到 trace
make dev LOG=info,acp=trace,store=debug
```

组件名与包名对应：`acp` `store` `api` `app` `gitx` `ghx` `fsstore` `eventbus`。

实现上是 `DUET_LOG` 环境变量，格式 `[全局级别,][组件=级别,]...`。

---

## 4. 每条日志必须能被关联

日志的价值在于**能把一件事的全过程串起来**。所以：

| 字段 | 来源 | 何时必带 |
|---|---|---|
| `component` | 包名，自动 | 总是 |
| `work_id` | context | 任何与某个 Work 相关的操作 |
| `unit_id` | context | 单元执行期间 |
| `attempt_id` | context | attempt 期间 |
| `trace_id` | context，一次 HTTP 请求 / 一轮 ACP turn 生成一个 | 总是 |

```go
// ✓ 从 context 自动带上关联字段
log := logging.FromContext(ctx)          // 已带 work_id/unit_id/trace_id
log.Info("单元契约已冻结", "version", 3)

// ✗ 手动拼，漏了就串不起来
slog.Info("contract frozen for " + workID)
```

**关联字段从 `context` 走，不手动传。** 在入口处（HTTP 中间件、ACP turn 开始）
用 `logging.With(ctx, "work_id", id)` 塞进去，下游自动继承。

---

## 5. 消息写法

```go
// ✓ 消息是固定的短句，可变部分放 attrs —— 这样才能按 msg 聚合统计
log.Error("ACP 会话取消超时", "session_id", sid, "elapsed_ms", 30000)

// ✗ 把变量拼进消息，每条都不一样，聚合不了
log.Error(fmt.Sprintf("session %s cancel timed out after 30s", sid))
```

| 规则 | |
|---|---|
| 消息用**中文短句**，固定不变 | 便于按 msg 聚合 |
| 可变部分一律进 attrs | `key` 用 `snake_case` |
| 错误用 `"err", err` | 不要 `err.Error()`，handler 会处理 |
| **禁止记录敏感值** | 令牌、密钥、用户代码内容 —— 见 §7 |

---

## 6. 落库的三条硬约束

### ① 不阻塞业务路径

日志写库走**异步缓冲 + 批量提交**。业务代码调 `log.Info()` 时只是往 channel 里塞，
立刻返回。

### ② 落库失败不能让业务失败

SQLite 忙、磁盘满、表被锁——都只降级为「只写 stderr」，**绝不向上抛错**。
日志系统挂掉不该让产品挂掉。

### ③ 不能无限增长

| 策略 | 值 |
|---|---|
| 保留时长 | 7 天 |
| 条数上限 | 200k |
| 清理时机 | 启动时 + 每小时 |

超限时**从最旧的删**，且 `ERROR` 级别保留更久（30 天）——
排查线上问题时最需要的就是老 ERROR。

> SQLite 是单写者，日志写入必须用**独立的短事务**，
> 不能和业务事务混在一起（见 `database.md` §6）。

---

## 7. 绝不记录的东西

- ✗ GitHub 令牌、API key、任何凭据（即使是片段）
- ✗ 用户代码内容 / diff 全文（记 hash 与行数，不记内容）
- ✗ ACP 的 prompt 全文（记长度与摘要；全文在运行记录里，那是另一套）
- ✗ 用户的文件路径中的家目录（脱敏成 `~/`）

**日志会被贴进 issue、发给别人看。** 按「这行会被公开」的标准写。

---

## 8. 前端日志

前端不落库（浏览器里没有我们的 SQLite）。规则：

| 环境 | 行为 |
|---|---|
| 开发 | `console.debug/info/warn/error`，带 component 前缀 |
| 生产 | 只保留 `warn` / `error`；**未捕获异常上报到后端** `POST /v1/system/client-errors` |

**不做前端日志采集系统。** 需要排查的前端问题基本都能在开发态复现——
为它建一套采集链路是过度设计。

---

## 9. AI 怎么用

```bash
make dev-logs                    # 跟踪 stderr（看生命周期）
make dev LOG=acp=trace           # 只把 acp 调到 trace 再复现一次
```

```sql
-- 落库的日志用 db-operate skill 查
SELECT ts, level, component, msg, attrs FROM logs
WHERE work_id = 'work-08' ORDER BY seq DESC LIMIT 50;

-- 某次 attempt 的全过程
SELECT ts, component, msg FROM logs WHERE attempt_id = 'att-03' ORDER BY seq;

-- 只看错误
SELECT ts, component, msg, attrs FROM logs WHERE level >= 8 ORDER BY seq DESC LIMIT 20;
```

**排查顺序**：先看 stderr 找时间点 → 再按 `trace_id` / `work_id` 查库看全过程 →
需要报文细节时把对应组件调到 `trace` 复现一次。

---

## 10. 禁止清单

- ✗ 用 `fmt.Println` / `log.Print` —— 一律 `log/slog`
- ✗ 把变量拼进消息（聚合不了）
- ✗ 在循环里打 `INFO`
- ✗ `ERROR` 用于「预期内的失败」（用户取消、404）
- ✗ 记录凭据、用户代码内容、prompt 全文
- ✗ 日志写库失败向上抛错
- ✗ 日志写入与业务事务混在一个事务里
- ✗ 前端建日志采集系统（过度设计）
