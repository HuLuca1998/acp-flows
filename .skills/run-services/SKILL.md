---
name: run-services
description: 需要把 Duet 跑起来时使用——起后端、起前端、在浏览器里看页面、排查「服务起不来 / 端口被占 / 页面 404」。触发场景：要验证一个改动在真实应用里的效果、要做界面走查、要看 duetd 的日志、用户说「跑起来看看」「起个服务」「打开页面」。**这是本仓库唯一允许的起服务方式**——不要裸跑 `go run` 或 `pnpm dev`。
---

# 起服务与页面测试

> **不要裸跑 `go run ./cmd/duetd` 或 `pnpm dev`。** 一律走 `scripts/services.sh`。

## 为什么有这条规矩

AI 调试时最常见的翻车方式是**狂起进程**：

```
起一个 → 忘了关 → 下次端口被占 → 换个端口再起 → 又忘了关 → …
```

结果是十几个进程挂在后台吃内存，而且**下一轮 AI 会连到一个陈旧的实例上**
排查半天——代码改了但连的是旧进程，症状完全对不上。

`services.sh` 用三件事挡住它：**端口写死** · **幂等启动** · **PID + 端口双重兜底的停止**。

---

## 四条命令

```bash
make dev            # = services.sh start all，起前后端
make dev-status     # 看谁在跑
make dev-stop       # ★ 用完必须停
make dev-logs       # 跟踪后端日志
```

或直接用脚本（可指定单个服务）：

```bash
scripts/services.sh start backend
scripts/services.sh restart frontend
scripts/services.sh logs backend
```

| | 端口 | 地址 |
|---|---|---|
| 后端 `duetd` | **7777** | http://127.0.0.1:7777 |
| 前端 `vite` | **5173** | http://localhost:5173 |

**端口写死，不许换。** 换端口是上面那个失败模式的第一步。

---

## 标准流程

```bash
make dev            # 1. 起（已在跑就复用，不重启）
make dev-status     # 2. 确认真的起来了
# ... 3. 做你的验证
make dev-stop       # 4. ★ 用完必须停
```

**第 4 步不能省。** 留着的进程会占端口、吃内存，还会误导下一轮 AI。

### 幂等：已在跑就复用

`start` 发现服务已在运行时**不会重启**，只打印一行「复用」。
这样反复调用是安全的，也不会打断你正在看的调试会话。

---

## 页面测试

起好之后用浏览器工具（`agent-browser` skill 或 `mcp__Claude_Browser__*`）：

```
1. preview_start  → http://localhost:5173
2. read_page      → 读可访问性树（★ 比截图更适合核对文本与结构）
3. computer       → 点击 / 输入 / 滚动 / 拖拽
4. read_console_messages / read_network_requests → 排查报错与请求
```

**核对文本与结构优先用 `read_page` 而不是截图**——截图看不出可访问名称，
而设计规范要求所有纯图标按钮都有 tooltip。

走查清单见 `web-ui-test` skill 第三层。

### 调 API

后端要 token（无 token 一律 401，这是刻意的）：

```bash
curl -H "Authorization: Bearer dev-local-token" \
     http://127.0.0.1:7777/v1/system/version
```

开发 token 固定是 `dev-local-token`，可用 `DUET_DEV_TOKEN` 覆盖。

---

## 排查

### 「端口 7777 被占用」

脚本会告诉你是哪个 pid、什么进程。**不要改端口绕开**：

```bash
make dev-stop         # 如果是我们之前没停干净的
kill <pid>            # 如果是别的程序
```

改端口能让这一次跑起来，但端口会越占越多，而且下次别人按文档用 7777 会连不上。

### 「启动超时」

`services.sh` 会自动打印日志末尾 15 行。也可以：

```bash
make dev-logs                     # 后端
scripts/services.sh logs frontend # 前端
```

日志在 `~/.duet-dev/run/{backend,frontend}.log`。

### 「改了代码但行为没变」

八成是连到了陈旧的实例。

```bash
make dev-status       # 看 pid 是不是你以为的那个
scripts/services.sh restart backend
```

> 后端是 `go run`，改代码**不会**自动重载，必须 restart。
> 前端 vite 有 HMR，改前端代码不用重启。

### 「页面 404 / 白屏」

先分清是哪一层：

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:5173        # 前端
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:7777/v1/system/version  # 后端（无 token 应为 401）
```

后端 401 是**正常的**——它证明服务活着且鉴权生效。

---

## 数据隔离（铁律 6）

开发态数据目录是 `~/.duet-dev`，**与用户真实的 `~/.acpflows` 完全隔离**。

要从零开始：

```bash
make dev-stop
rm -rf ~/.duet-dev/duet.db*    # 只删数据库，保留日志与 pid 目录
make dev
```

**绝不要碰 `~/.acpflows`。** 那是用户的真实工作记录。

---

## 禁止清单

- ✗ 裸跑 `go run ./cmd/duetd` 或 `pnpm dev`（绕过了幂等与 PID 记账）
- ✗ 端口被占时换一个端口
- ✗ 用完不停（`make dev-stop` 是流程的一部分，不是可选的）
- ✗ 用 `pkill -f node` 这类粗暴清理（会误杀用户的其他进程）
- ✗ 碰 `~/.acpflows`
- ✗ 只截图不 `read_page` 就断言界面对了
