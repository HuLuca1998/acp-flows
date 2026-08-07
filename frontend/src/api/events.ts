import { authHeader } from "./client";
import type { components } from "./gen/schema";


/** 一条事件。形状由 api/openapi.yaml 决定。 */
export type StreamEvent = components["schemas"]["Event"];

/**
 * 事件流客户端（SSE）。
 *
 * ★ **这里是全前端唯一允许裸 `fetch` 的地方**，因为 SSE 走不了别的路：
 *
 * - `openapi-fetch` 处理的是一次性 JSON 响应，它不吐流。
 * - 浏览器原生的 `EventSource` **带不了 `Authorization` 头**，
 *   而后端对所有端点都要 bearer token（回环上任何本机进程都能连，
 *   没 token 等于谁都能驱动 Agent 改用户代码）。第一版用的正是 EventSource，
 *   单测全绿而真机上 `/v1/events` 一路 401——时间线永远是空的。
 *
 * 把 token 挪进查询串是更省事的修法，但 URL 会进浏览器历史与访问日志，
 * 而这个 token 等于「驱动 Agent 改用户代码」的权限。
 *
 * 代价是重连与 `Last-Event-ID` 要自己带。续传本身不复杂：记住最后一个 seq，
 * 重连时放进请求头，服务端只补它之后的（见 backend/internal/api/events.go）。
 */

/** 连一次并一直读到断开。正常结束（服务端关流）也会返回。 */
export async function readEventStream(
  signal: AbortSignal,
  lastSeq: number,
  onEvent: (e: StreamEvent) => void,
): Promise<void> {
  const res = await fetch("/v1/events", {
    signal,
    headers: {
      ...authHeader(),
      Accept: "text/event-stream",
      // 续传游标：服务端只补**它之后**的。0 表示「我一条都没有，从头给我」；
      // 重连时是最后收到的 seq——重发整条时间线的话，
      // 用户会看到所有内容再演一遍。
      "Last-Event-ID": String(lastSeq),
    },
  });
  if (!res.ok || res.body === null) {
    throw new Error(`events stream: ${res.status}`);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  for (;;) {
    const { done, value } = await reader.read();
    if (done) {
      return;
    }
    // stream: true 不能省——一个多字节字符可能横跨两个网络分片，
    // 不带的话中文会被解成乱码。
    buffer += decoder.decode(value, { stream: true });

    // SSE 用空行分隔消息块
    let sep = buffer.indexOf("\n\n");
    while (sep !== -1) {
      const parsed = parseBlock(buffer.slice(0, sep));
      buffer = buffer.slice(sep + 2);
      if (parsed !== null) {
        onEvent(parsed);
      }
      sep = buffer.indexOf("\n\n");
    }
  }
}

/** 从一个 SSE 消息块里取出事件；取不到（心跳、坏 JSON）返回 null。 */
function parseBlock(block: string): StreamEvent | null {
  const data = block
    .split("\n")
    // 注释行（心跳）以冒号开头，直接跳过
    .filter((line) => line.startsWith("data:"))
    .map((line) => line.slice(5).trimStart())
    .join("\n");

  if (data === "") {
    return null;
  }
  try {
    return JSON.parse(data) as StreamEvent;
  } catch {
    // 一条坏消息不该让整条流断掉：跳过它，后面的照常
    return null;
  }
}
