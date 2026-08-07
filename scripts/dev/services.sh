#!/usr/bin/env bash
# 本地服务管理：固定端口、幂等启动、干净关闭。
#
# 存在的理由：AI 调试时最常见的翻车方式是**狂起进程**——
# 起一个、忘了关；端口被占了就换一个端口再起；最后十几个进程挂在后台，
# 内存吃满，下一轮 AI 连到一个陈旧的实例上排查了半天。
#
# 所以这里只做三件事，且是**唯一**允许的起服务方式：
#   · 端口写死（duetd 7777 / vite 5173），不许换
#   · 幂等：已经在跑就复用，不重启
#   · PID 文件 + 端口双重记账，stop 一定能停干净
#
#   scripts/dev/services.sh start [backend|frontend|all]
#   scripts/dev/services.sh stop  [backend|frontend|all]
#   scripts/dev/services.sh status
#   scripts/dev/services.sh restart [...]
#   scripts/dev/services.sh logs backend
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

# ★ 后台进程必须 </dev/null 且不继承父进程的 stdout —— 否则它会一直持有
# 调用方的管道，表现为「make dev 挂住不返回」。踩过一次。
#
# ★ 端口写死。换端口是上面说的那个失败模式的第一步，不要开这个口子。
readonly BACKEND_PORT=7777
readonly FRONTEND_PORT=5173

readonly RUN_DIR="${DUET_RUN_DIR:-$HOME/.duet-dev/run}"
readonly DATA_DIR="${DUET_DATA_DIR:-$HOME/.duet-dev}"
mkdir -p "$RUN_DIR"

pid_file()  { echo "$RUN_DIR/$1.pid"; }
log_file()  { echo "$RUN_DIR/$1.log"; }
port_of()   { [[ $1 == backend ]] && echo "$BACKEND_PORT" || echo "$FRONTEND_PORT"; }

# 端口上是否有东西在听，有的话是谁
port_holder() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -1 || true
}

is_running() {
  local svc=$1 pf; pf=$(pid_file "$svc")
  [[ -f $pf ]] || return 1
  local pid; pid=$(cat "$pf" 2>/dev/null || echo "")
  [[ -n $pid ]] && kill -0 "$pid" 2>/dev/null
}

start_one() {
  local svc=$1 port; port=$(port_of "$svc")

  # 幂等：已经在跑就复用。**不要重启** —— 重启会打断正在看的调试会话。
  if is_running "$svc"; then
    echo "· $svc 已在运行（pid $(cat "$(pid_file "$svc")")，端口 $port）—— 复用"
    return 0
  fi

  # 端口被别的东西占着：报告是谁，让人决定，不要换端口绕开
  local holder; holder=$(port_holder "$port")
  if [[ -n $holder ]]; then
    echo "✗ 端口 $port 被 pid $holder 占用：$(ps -p "$holder" -o comm= 2>/dev/null || echo 未知)" >&2
    echo "  这多半是之前没停干净的实例。停掉它：" >&2
    echo "    scripts/dev/services.sh stop $svc      # 如果是我们的" >&2
    echo "    kill $holder                        # 如果是别的" >&2
    echo "  ★ 不要改端口绕开 —— 那正是端口越占越多的原因。" >&2
    return 1
  fi

  local log; log=$(log_file "$svc")
  case "$svc" in
    backend)
      [[ -f backend/go.mod ]] || { echo "✗ backend/go.mod 不存在"; return 1; }
      ( cd backend && DUET_DATA_DIR="$DATA_DIR" DUET_LOG="${DUET_LOG:-info}" \
          nohup go run ./cmd/duetd -dev </dev/null >"$log" 2>&1 &
        echo $! >"$(pid_file backend)" ) </dev/null
      ;;
    frontend)
      [[ -f frontend/package.json ]] || { echo "✗ frontend/package.json 不存在"; return 1; }
      ( cd frontend && nohup pnpm dev --port "$FRONTEND_PORT" --strictPort \
          </dev/null >"$log" 2>&1 &
        echo $! >"$(pid_file frontend)" ) </dev/null
      ;;
    *) echo "✗ 未知服务: $svc" >&2; return 1 ;;
  esac

  # 等它真的起来，而不是「命令返回了就算起来了」
  for _ in $(seq 1 40); do
    [[ -n $(port_holder "$port") ]] && { echo "✓ $svc 已启动（端口 $port，日志 $log）"; return 0; }
    sleep 0.5
  done
  echo "✗ $svc 启动超时（20s）。日志末尾：" >&2
  tail -15 "$log" >&2
  stop_one "$svc" >/dev/null 2>&1 || true
  return 1
}

stop_one() {
  local svc=$1 port; port=$(port_of "$svc")
  local pf; pf=$(pid_file "$svc")
  local stopped=0

  # 先按 PID 文件停
  if [[ -f $pf ]]; then
    local pid; pid=$(cat "$pf" 2>/dev/null || echo "")
    if [[ -n $pid ]] && kill -0 "$pid" 2>/dev/null; then
      # 杀进程组：go run 会 fork 出真正的二进制，pnpm dev 会 fork esbuild，
      # 只杀父进程会留下孤儿继续占端口。
      kill -TERM -- "-$(ps -o pgid= "$pid" | tr -d ' ')" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
      for _ in $(seq 1 20); do kill -0 "$pid" 2>/dev/null || break; sleep 0.25; done
      kill -KILL "$pid" 2>/dev/null || true
      stopped=1
    fi
    rm -f "$pf"
  fi

  # 再按端口兜底 —— PID 文件可能因为崩溃而失联
  local holder; holder=$(port_holder "$port")
  if [[ -n $holder ]]; then
    kill -TERM "$holder" 2>/dev/null || true
    sleep 1
    holder=$(port_holder "$port")
    [[ -n $holder ]] && kill -KILL "$holder" 2>/dev/null || true
    stopped=1
  fi

  [[ $stopped -eq 1 ]] && echo "✓ $svc 已停止" || echo "· $svc 未在运行"
}

status() {
  printf '%-10s %-8s %-8s %s\n' 服务 端口 状态 地址
  for svc in backend frontend; do
    local port; port=$(port_of "$svc")
    local holder; holder=$(port_holder "$port")
    local state url
    if is_running "$svc"; then
      state="运行中"
    elif [[ -n $holder ]]; then
      state="被占用"   # 端口上有东西但不是我们起的
    else
      state="未运行"
    fi
    [[ $svc == backend ]] && url="http://127.0.0.1:$port" || url="http://localhost:$port"
    printf '%-10s %-8s %-8s %s\n' "$svc" "$port" "$state" "$url"
  done
  echo
  echo "日志级别:   ${DUET_LOG:-info}   （改：make dev LOG=acp=trace）"
  echo "开发 token: ${DUET_DEV_TOKEN:-dev-local-token}"
  echo "数据目录:   $DATA_DIR   （与用户真实的 ~/.acpflows 隔离）"
}

expand() { [[ ${1:-all} == all ]] && echo "backend frontend" || echo "$1"; }

case "${1:-}" in
  start)   for s in $(expand "${2:-all}"); do start_one "$s"; done ;;
  stop)    for s in $(expand "${2:-all}"); do stop_one  "$s"; done ;;
  restart) for s in $(expand "${2:-all}"); do stop_one "$s"; start_one "$s"; done ;;
  status)  status ;;
  logs)    tail -f "$(log_file "${2:-backend}")" ;;
  *)
    cat >&2 <<EOF
用法: scripts/dev/services.sh <命令> [backend|frontend|all]

  start    启动（幂等：已在跑就复用，不重启）
  stop     停止（PID + 端口双重兜底，杀进程组防孤儿）
  restart  重启
  status   看谁在跑
  logs     跟踪日志： scripts/dev/services.sh logs backend

端口写死：backend $BACKEND_PORT · frontend $FRONTEND_PORT
EOF
    exit 2 ;;
esac
