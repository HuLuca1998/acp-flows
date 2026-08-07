import { useMemo } from "react";
import { useTranslation } from "react-i18next";

import styles from "./Timeline.module.css";
import { rendererFor, type TimelineEvent } from "./event-registry";

export type TimelineProps = {
  events: TimelineEvent[];
  /** 被关掉的事件类型；不传表示全显示。 */
  hidden?: ReadonlySet<string>;
};

/** 合并后的一段。 */
type Segment = {
  key: string;
  type: string;
  /** 合并进来的全部文本 */
  text: string;
  /** 摘要：这一条到底在干什么（工具调用的标题、文件路径……） */
  detail: string;
  /** 当前状态，取最后一次更新 */
  status: string;
  /**
   * 摘要来自 detailFrom 的第几项，**越小越好**。
   *
   * ★ 归并时靠它挡住「降级覆盖」：tool_call 带 title、随后的
   * tool_call_update 只带 kind，直接覆盖的话卡片上会显示
   * 「tool_call_update」而不是「Read README.md」——用户看不出 AI 在读哪个文件。
   */
  detailRank: number;
  /** 段内最后一条事件的序号，用来做 key 与调试 */
  lastSeq: number;
  count: number;
};

/**
 * 时间线（验收点 V6）。
 *
 * ★ 渲染形态全部来自 `event-registry`，**这里没有一个 switch**
 * （`U2.3.2` 的 forbidden_changes 明写禁止）。加一类事件只加一条注册。
 */
export function Timeline({ events, hidden }: TimelineProps) {
  const { t } = useTranslation();

  const segments = useMemo(() => mergeEvents(events, hidden), [events, hidden]);

  if (segments.length === 0) {
    return <p className={styles.empty}>{t("timeline.empty")}</p>;
  }

  return (
    <div className={styles.list}>
      {segments.map((seg) => {
        const renderer = rendererFor(seg.type);
        return (
          <div
            key={seg.key}
            className={`${styles.item} ${styles[renderer.shape]}`}
            data-event-type={seg.type}
            data-shape={renderer.shape}
            data-status={seg.status === "" ? undefined : seg.status}
          >
            <span className={styles.label}>{t(renderer.labelKey)}</span>
            {seg.detail !== "" && (
              <span className={styles.detail}>{seg.detail}</span>
            )}
            <span className={styles.text}>{seg.text}</span>
          </div>
        );
      })}
    </div>
  );
}

/**
 * 把事件列表合并成显示用的段。
 *
 * ★ **只有 `merge: true` 的类型才合并**（文本流）。工具调用两次就是两次——
 * 合并的话用户会以为 AI 只动了一个文件。
 *
 * 合并的意义在于「不闪烁」：流式文本一个字一个字地来，每片一个气泡的话，
 * 界面会在打字过程中疯狂重排。
 */
function mergeEvents(
  events: TimelineEvent[],
  hidden?: ReadonlySet<string>,
): Segment[] {
  const out: Segment[] = [];
  // 按 mergeKey 索引已经开出来的段，供后续的状态更新找回去
  const byKey = new Map<string, Segment>();

  for (const e of events) {
    const type = e.type ?? "";
    if (hidden?.has(type) === true) {
      continue;
    }

    const renderer = rendererFor(type);
    const payload: Record<string, unknown> = e.payload ?? {};
    const text = textOf(e);
    const [detail, detailRank] = pickFirst(payload, renderer.detailFrom);
    const status =
      renderer.statusFrom === undefined
        ? ""
        : stringAt(payload, renderer.statusFrom);

    // ★ 按业务对象归并：同一次工具调用的开始与若干次状态更新是一张卡片。
    // 中间可以隔着别的事件，所以查的是 map 而不是「上一条」。
    const mergeID =
      renderer.mergeKey === undefined
        ? ""
        : stringAt(payload, renderer.mergeKey);
    if (mergeID !== "") {
      const existing = byKey.get(`${type}:${mergeID}`);
      if (existing !== undefined) {
        existing.text += text;
        // 后来的补充先前的，但**不许降级**——见 Segment.detailRank。
        // 同一档要覆盖（<= 而不是 <）：ACP 先给泛称「Read File」，
        // 随后的 update 才补上具体的「Read README.md」，两者都在 title 上。
        if (detail !== "" && detailRank <= existing.detailRank) {
          existing.detail = detail;
          existing.detailRank = detailRank;
        }
        if (status !== "") existing.status = status;
        existing.lastSeq = e.seq ?? existing.lastSeq;
        existing.count += 1;
        continue;
      }
    }

    // 连续同类的文本流并进同一个气泡（流式消息）
    const last = out[out.length - 1];
    if (renderer.merge === true && last !== undefined && last.type === type) {
      last.text += text;
      last.lastSeq = e.seq ?? last.lastSeq;
      last.count += 1;
      continue;
    }

    const seg: Segment = {
      key: `${type}-${e.seq ?? out.length}`,
      type,
      text,
      detail,
      detailRank,
      status,
      lastSeq: e.seq ?? 0,
      count: 1,
    };
    out.push(seg);
    if (mergeID !== "") {
      byKey.set(`${type}:${mergeID}`, seg);
    }
  }

  return out;
}

/**
 * 按注册表声明的顺序取第一个有值的字段，并返回它的**名次**。
 *
 * 名次用来挡住归并时的降级覆盖（见 Segment.detailRank）。
 * 支持 `a.b` 与 `a.0.b` 形式的路径——载荷是 Agent 给的，形状我们说了不算。
 */
function pickFirst(
  payload: Record<string, unknown>,
  paths?: readonly string[],
): [detail: string, rank: number] {
  const list = paths ?? [];
  for (let i = 0; i < list.length; i += 1) {
    const v = stringAt(payload, list[i] ?? "");
    if (v !== "") {
      return [v, i];
    }
  }
  return ["", Number.MAX_SAFE_INTEGER];
}

/** 按路径取一个字符串；取不到返回空串，**绝不抛**。 */
function stringAt(payload: Record<string, unknown>, path: string): string {
  let cur: unknown = payload;
  for (const part of path.split(".")) {
    if (Array.isArray(cur)) {
      cur = cur[Number(part)];
      continue;
    }
    if (cur === null || typeof cur !== "object") {
      return "";
    }
    cur = (cur as Record<string, unknown>)[part];
  }
  return typeof cur === "string" ? cur : "";
}

/**
 * 从载荷里取要显示的文本。
 *
 * 取不到时返回空串而不是抛——**一条载荷形状意外的事件不该让整个时间线白屏**。
 * 后端加字段、改结构时，用户看到的应该是「这条少了点东西」而不是整页没了。
 */
function textOf(e: TimelineEvent): string {
  const payload = e.payload as Record<string, unknown> | undefined;
  if (payload === undefined) {
    return "";
  }
  const text = payload.text;
  return typeof text === "string" ? text : "";
}
