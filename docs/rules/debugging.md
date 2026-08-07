# 排查手册

> 流程与第一原则在 [`debug` skill](../../.skills/debug/SKILL.md)（先读那个）。
> **本文是查询与手法的速查表**，按需 grep，不用整篇读。
> 日志本身的规范见 [`logging.md`](logging.md)。

```bash
grep -n '^## ' docs/rules/debugging.md      # 看有哪些节
```

---

## 1. 落库日志的查询速查

连库（路径见 [`db-operate` skill](../../.skills/db-operate/SKILL.md)）：

```bash
sqlite3 -header -box ~/.duet-dev/.acpflows/duet.db
```

`level` 是 slog 数值：`trace -8` · `debug -4` · `info 0` · `warn 4` · `error 8`。

```sql
-- 这个 Work 的全过程（最高频）
SELECT ts, component, msg, attrs FROM logs
WHERE work_id = 'work-08' ORDER BY seq;

-- 一次 attempt 里发生了什么
SELECT ts, component, msg FROM logs WHERE attempt_id = 'att-03' ORDER BY seq;

-- 一次请求 / 一轮 turn 的全链路（trace_id 串起跨组件调用）
SELECT ts, component, msg FROM logs WHERE trace_id = 'tr-xxx' ORDER BY seq;

-- 只看错误
SELECT ts, component, msg, attrs FROM logs WHERE level >= 8 ORDER BY seq DESC LIMIT 20;

-- 只看某个组件
SELECT ts, level, msg FROM logs WHERE component = 'acp' ORDER BY seq DESC LIMIT 50;

-- 有哪些组件在打日志（不知道该调哪个域时先跑这条）
SELECT component, COUNT(*) FROM logs GROUP BY component ORDER BY 2 DESC;

-- ★ 出错前后各 20 条：看它是被什么打断的
SELECT ts, level, component, msg FROM logs
WHERE seq BETWEEN (SELECT seq FROM logs WHERE level >= 8 ORDER BY seq DESC LIMIT 1) - 20
              AND (SELECT seq FROM logs WHERE level >= 8 ORDER BY seq DESC LIMIT 1) + 20
ORDER BY seq;

-- attrs 是 JSON，可以直接提字段
SELECT ts, msg, json_extract(attrs, '$.stop_reason') AS reason
FROM logs WHERE msg = 'turn 结束' ORDER BY seq DESC LIMIT 20;
```

**关联字段是查询的钥匙**：`work_id` `unit_id` `attempt_id` `trace_id`，
从 context 自动继承。**如果一件事串不起来，说明有代码没从 context 取 logger
——那本身是缺陷，顺手修掉。**

---

## 2. 加临时调试日志

```go
// ★ 用 context 里的 logger，关联字段自动带上
logging.FromContext(ctx).Log(ctx, logging.LevelTrace, "契约冻结前的状态",
    "unit_id", u.ID, "version", u.Version, "state", u.State)
```

三条：

1. **`logging.FromContext(ctx)`，不要 `slog.Default()`** —— 后者没有关联字段，
   查的时候串不起来，等于白加
2. **用 `LevelTrace`** —— 落库但不进 stderr，不污染别人的观测面
3. **值放 attr，不要拼进 message** —— `"契约冻结", "version", 3`
   而不是 `fmt.Sprintf("契约 v%d 冻结", 3)`。拼进去就没法按 message 聚合

加完：

```bash
make dev-restart LOG=<组件>=trace     # ★ 必须重启，go run 不热重载
# 复现一次，然后查库
```

### 排查完留不留

| 情况 | 动作 |
|---|---|
| 对**理解正常流程**也有价值 | 留下，定 DEBUG 或 TRACE |
| 纯粹为了这次排查 | 删掉 |
| 你发现「这里本来就该有日志」 | 留下并写进提交说明 |

**不要留一堆 `here1` `here2` 式的痕迹。** 那是噪音，会让下一个人以为它们有意义。

---

## 3. 二分定位

```bash
git bisect start
git bisect bad                        # 现在是坏的
git bisect good <某个好的提交>
git bisect run make test-backend      # 能自动判定就让它跑完
git bisect reset
```

**「以前是好的」这个信息价值极高**，别浪费它——比读代码快得多。

---

## 4. 最小复现测试

**常常是最快的路**，而且修完自然就是回归防线：

```bash
cd backend && go test ./internal/store -run TestWorkRepo_R7 -v -count=1
cd backend && go test ./internal/acp/... -run TestConn -race -count=1
```

比在完整应用里复现快一个数量级。

> ⚠️ 涉及 pipe / channel / 子进程 stdio 的测试要给读取加超时，
> 否则表现为**挂住**而不是失败（pitfalls P-08）。

---

## 5. ACP 层：用探针，不要推理

```bash
make probe        # 零模型开销，结果写进 backend/tests/fixtures/probe/
```

对照 [`../notes/acp-field-notes.md`](../notes/acp-field-notes.md) §7.1。
**设计稿说的和探针说的冲突时，以探针为准。**

---

## 6. HTTP / SSE 层

```bash
TOKEN=$(jq -r .token ~/.duet-dev/.acpflows/runtime/session.json)

curl -sv -H "Authorization: Bearer $TOKEN" \
     http://127.0.0.1:7777/api/v1/system/version | jq

# SSE：看事件到底发出来没有
curl -sN -H "Authorization: Bearer $TOKEN" \
     http://127.0.0.1:7777/api/v1/works/work-08/events
```

**前端说「没收到事件」时，先用 curl 确认后端到底发没发。**
这一刀把问题切成前端问题或后端问题，别在中间猜。

---

## 7. 前端

浏览器控制台、网络请求走 [`web-ui-test` skill](../../.skills/web-ui-test/SKILL.md)。

**「代码看着对但页面不对」**：先看网络请求的**真实响应体**，
再看组件收到的 props —— 十有八九是数据问题，不是渲染问题。

---

## 8. 数据库

「数据对不上」有独立的排查顺序，走 [`db-operate` skill](../../.skills/db-operate/SKILL.md)。

第一条永远是 `PRAGMA foreign_keys`，
**但在 `sqlite3` CLI 里它永远返回 0，那是正常的**（pitfalls P-17）——
pragma 是每连接的，CLI 自己的连接没带 DSN 参数。要验证应用侧跑
`TestOpen_R1_ForeignKeysPragmaIsOn`。
