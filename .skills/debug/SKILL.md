---
name: debug
description: 排查问题、定位缺陷时使用。触发场景：测试红了但不知道为什么、页面行为不对、接口返回了意外结果、服务起不来、某个流程"看起来跑了但没效果"、用户说「查一下为什么」「排查一下」。覆盖查日志（stderr 与落库）、按域调级别、加临时调试日志、以及日志之外的定位手段。**不负责修复本身**——定位清楚后按 tdd-unit 走。
---

# 排查与调试

> **AI 只能看日志。** 人可以 attach 调试器、盯着界面看、凭手感猜；你没有这些。
> 所以本项目对可观测性的要求比一般项目高，而你必须会用它。
>
> 本文是**流程**。具体查询语句、各层手法在
> [`docs/rules/debugging.md`](../../docs/rules/debugging.md)（按需 grep，别整篇读）。

## 第一原则：先复现，再解释

**不要看着代码推理原因。** 推理出来的原因有一半是错的，而且错得很自信。

```
观察到现象 → 让它可复现 → 缩小范围 → 找到根因 → 写一个能复现它的测试 → 修
```

**最后那步不能省。** 手动查明白但没留下测试，下一轮 AI 会再踩一次。

---

## ① 三十秒自检 —— 先确认你观察的对象是对的

```bash
make dev-status        # 服务在跑吗？跑的是哪个进程？什么时候起的？
git status             # 你改的代码提交了吗？
```

| 症状 | 十有八九是 |
|---|---|
| 改了后端代码但行为没变 | **没重启** —— `go run` 不热重载，跑 `make dev-restart` |
| 页面还是旧的 | 浏览器缓存 / vite 没热更，硬刷新 |
| 「日志我明明加了但没打出来」 | 级别不够（见 ③），或者连的是旧进程 |
| 端口被占、连上了但数据不对 | 有僵尸进程，`make dev-stop` 再 `make dev` |

**★ 最坑的一种：代码改了但连的是旧进程。** 症状与代码完全对不上，能耗掉半小时。
所以第一条永远是看进程启动时间。

---

## ② 拿着报错原文查踩坑档案

```bash
grep -n "你看到的报错原文" docs/notes/pitfalls.md
```

**最便宜、命中率最高的一步。** 档案里每一条都是本项目真实花过时间的坑，
命中一条能省几十分钟。找不到再往下走。

---

## ③ 看日志

两个去处，**分工不同，别用错**：

| 去处 | 内容 | 怎么看 | 回答什么 |
|---|---|---|---|
| **stderr** | INFO 以上，生命周期 | `make dev-logs` | 「它跑到哪一步了」 |
| **SQLite `logs` 表** | **全部级别**，含 TRACE | `make logs-db` / SQL | 「全过程发生了什么」 |

> ★ **TRACE 永远不进 stderr**（报文全文量太大，会把生命周期日志完全淹掉）。
> 要看协议报文、SQL 语句、逐帧事件，**只能查库**。

```bash
make dev-logs                  # 先在 stderr 上定位到时间点
make logs-db                   # 最近 30 条
```

复杂查询（按 work_id / trace_id 串全过程、出错前后各 20 条、按组件统计）
见 [`docs/rules/debugging.md`](../../docs/rules/debugging.md) §1。

### 按域调级别，不要全局调 debug

```bash
make dev LOG=acp=trace                 # 只把 ACP 层调到 trace ✓
make dev LOG=info,acp=trace,store=debug
make dev LOG=debug                     # 全局 —— 几乎总是错的选择 ✗
```

**全局 debug 会淹没在噪音里**，你要找的那一行会被冲走。
先想清楚「问题在哪一层」，只调那一层。不知道有哪些组件就查一下
（`SELECT component, COUNT(*) FROM logs GROUP BY component;`）。

---

## ④ 加调试日志

**加在「我以为是 X，想确认真的是 X」的地方**，不是加在报错的地方——
报错的地方你已经知道了，不确定的是它上游的状态。

```go
logging.FromContext(ctx).Log(ctx, logging.LevelTrace, "契约冻结前的状态",
    "unit_id", u.ID, "version", u.Version, "state", u.State)
```

- **用 `logging.FromContext(ctx)`，不要 `slog.Default()`** —— 后者丢关联字段，查的时候串不起来
- **用 `LevelTrace`** —— 落库但不进 stderr
- **值放 attr，不要拼进 message**

加完必须 `make dev-restart LOG=<组件>=trace` 再复现。
写法细节与「排查完留不留」见 [`debugging.md`](../../docs/rules/debugging.md) §2。

---

## ⑤ 日志之外 —— 常常更快

| 手段 | 什么时候用 | 详见 |
|---|---|---|
| **`git bisect`** | **「以前是好的」** —— 这个信息价值极高，别浪费 | [§3](../../docs/rules/debugging.md) |
| **最小复现测试** | 几乎总是最快的路，且修完就是回归防线 | [§4](../../docs/rules/debugging.md) |
| `make probe` | ACP Runtime 行为存疑 —— **不要推理，去问真机** | [§5](../../docs/rules/debugging.md) |
| `curl` 打接口 / SSE | 前端说「没收到事件」时，**先确认后端发没发** | [§6](../../docs/rules/debugging.md) |
| 浏览器工具 | 「代码看着对但页面不对」 | `web-ui-test` skill |
| SQL | 「数据对不上」 | `db-operate` skill |

---

## ⑥ 收尾

**定位到了 → 先写一个能复现它的失败测试，确认它红，再修。**
「我已经知道怎么修了，直接改吧」是最常见的破窗方式——
没有那个先红的测试，你无法证明修的是这个问题。

**花了 >15 分钟且会再犯的坑 → 归档**，按
[`docs/notes/pitfalls.md`](../../docs/notes/pitfalls.md) 的维护规则。
**但先问：能不能变成检查？** 能变成检查脚本 / 测试 / lint 的就不要写进文档——
文档挡不住重复踩坑，档案里的 P-01 记录在案还是被踩了 4 次。

**排查不下去** → 按 [`ai-playbook.md`](../../docs/ai-playbook.md) §4：
踩坑档案 → open-questions → adr → 前作 `~/work/ai-workflows` → 实测 → **问人**。
不许猜一个原因当成事实往下改。

---

## 禁止清单

- ✗ `fmt.Println` / `log.Print` 调试 —— 一律 `log/slog`
- ✗ `slog.Default()` 加调试日志（丢关联字段，串不起来）
- ✗ 全局 `LOG=debug` 然后在噪音里翻
- ✗ 看着代码推理原因，不复现就动手改
- ✗ 排查完留一堆 `here1` `here2`
- ✗ 修完不写测试 —— 那不叫修好，只是这次好了
- ✗ 在 `~/.acpflows`（用户真实数据）上排查 —— 用 `~/.duet-dev`
- ✗ 日志里记凭据、用户代码内容、prompt 全文
