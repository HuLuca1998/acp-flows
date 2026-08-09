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
  /**
   * 这一段是**谁说的**。
   *
   * ★★ 来自事件载荷，**不在前端按 Runtime 名猜**：一个 Runtime 可以
   * 承担多个角色（`claude` 同时是需求分析师和审查员），按名字猜的话
   * 界面上两个角色会长得一模一样——而用户正是靠这个标签判断
   * 「现在是谁在说话、他能不能动我的文件」。
   */
  role: string;
  roleName: string;
  runtime: string;
  /**
   * 说这句话的时候，需求是第几版、冻结了没有。
   *
   * ★★ 同样来自事件载荷，**不在前端另查一次**：另查拿到的是「现在」的
   * 版本，而用户看的是一条历史消息——他会以为当时就已经是 v3 了。
   *
   * 0 表示这个工作还没有需求快照，那时不显示这枚标签
   * （显示一个「v0」比不显示更糟）。
   */
  reqVersion: number;
  reqFrozen: boolean;
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
      {segments.map((seg, i) => {
        const renderer = rendererFor(seg.type);
        // ★★ **连续同一个角色只在第一条显示头像与标签**（照设计稿）。
        //
        // 十条连续的工具调用每条都重复「claude · 需求分析师 · requirement v1」
        // 的话，那三个标签占掉半行宽度，而它们完全一样——
        // 用户要找的内容被自己的重复挤到了右边。
        const prev = segments[i - 1];
        const sameSpeaker =
          prev !== undefined &&
          prev.role === seg.role &&
          prev.reqVersion === seg.reqVersion &&
          prev.reqFrozen === seg.reqFrozen;
        return (
          <div
            key={seg.key}
            className={styles.row}
            data-event-type={seg.type}
            data-shape={renderer.shape}
            data-align={renderer.align ?? "start"}
            data-status={seg.status === "" ? undefined : seg.status}
          >
            {/*
              ★ 角色标签在最前面，形态照设计稿：`Claude · 需求分析师`。
              没有角色的事件（state_change、checkpoint 这些应用自己发的）
              **不显示这一块**——填个「系统」上去会让用户以为
              有个叫「系统」的角色在干活。
            */}
            {/*
              ★★ 头像方块，照设计稿：`CL` / `CX`。
              一屏扫过去**不读文字**就分得清哪几条是同一个人说的——
              只有标签的话，用户要逐条读文字才认得出来。
            */}
            {seg.roleName !== "" && !sameSpeaker && (
              <span
                className={styles.avatar}
                data-runtime={seg.runtime === "" ? undefined : seg.runtime}
                aria-hidden="true"
              >
                {initialsOf(seg.runtime)}
              </span>
            )}
            <div className={`${styles.item} ${styles[renderer.shape]}`}>
            {seg.roleName !== "" && sameSpeaker && (
              // 同一个人接着说：头像的位置留白，让内容对齐
              <span className={styles.avatarGap} aria-hidden="true" />
            )}
            {seg.roleName !== "" && !sameSpeaker && (
              <span className={styles.role} data-role={seg.role}>
                {seg.runtime !== "" && (
                  <span className={styles.runtime}>{seg.runtime}</span>
                )}
                {seg.roleName}
              </span>
            )}
            {/*
              ★ 需求版本标签，形态照设计稿：`requirement v2 已冻结`。
              `requirement v2` 是标识不翻译，「已冻结」是状态要翻译。
              没有需求快照（0）时**整块不显示**——「v0」比不显示更糟。
            */}
            {seg.reqVersion > 0 && !sameSpeaker && (
              <span className={styles.requirement} data-frozen={seg.reqFrozen}>
                {`requirement v${seg.reqVersion}`}
                {seg.reqFrozen && (
                  <span className={styles.frozen}>{t("timeline.requirementFrozen")}</span>
                )}
              </span>
            )}
            <span className={styles.label}>{t(renderer.labelKey)}</span>
            {seg.detail !== "" && (
              <span className={styles.detail}>{seg.detail}</span>
            )}
            <span className={styles.text}>{seg.text}</span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

/**
 * 设计稿定死的那两个缩写。
 *
 * ★★ `CL`（claude）与 `CX`（codex）是**人取的**，没有统一规则——
 * 一个取首辅音、一个取尾辅音。硬凑一条规则同时满足两者的话，
 * 那条规则会在第三个 Runtime 上给出谁都想不到的结果。
 */
const RUNTIME_INITIALS: Record<string, string> = {
  claude: "CL",
  codex: "CX",
};

/**
 * 头像上的两个字母。
 *
 * ★★ 对照表**只管设计稿定死的那两个**，其余走推导——
 * 只有对照表的话，加一个 Runtime 时忘了补一行，那个 Runtime 的头像
 * 会是空白，而用户看到的是一个没有身份的方块。
 */
function initialsOf(runtime: string): string {
  const name = runtime.trim().toLowerCase();
  if (name === "") {
    return "··";
  }
  const known = RUNTIME_INITIALS[name];
  if (known !== undefined) {
    return known;
  }
  // 兜底：首字母 + 最后一个辅音，拼不出就用前两个字符
  const first = name[0] ?? "";
  const rest = name.slice(1);
  const consonants = rest.replace(/[aeiou\d\W_]/g, "");
  const second = consonants.slice(-1) || rest[0] || first;
  return (first + second).toUpperCase();
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

    // 连续同类的文本流并进同一个气泡（流式消息）。
    //
    // ★★ **角色不同就不能并**：那是两个人在说话。并进去的话，
    // 需求分析师和实现工程师的话会挤在同一个气泡里，
    // 而标签只剩一个——用户分不清哪句是谁说的。
    //
    // ★★ **需求版本不同也不能并**，同一个道理：并进去的话标签只剩一个，
    // 用户看不出「哪句话之后需求变成了 v2」——而那正是他要找的分界。
    const last = out[out.length - 1];
    if (
      renderer.merge === true &&
      last !== undefined &&
      last.type === type &&
      last.role === (e.role ?? "") &&
      last.reqVersion === (e.requirement_version ?? 0) &&
      last.reqFrozen === (e.requirement_frozen ?? false)
    ) {
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
      role: e.role ?? "",
      roleName: e.role_display_name ?? "",
      runtime: e.runtime ?? "",
      reqVersion: e.requirement_version ?? 0,
      reqFrozen: e.requirement_frozen ?? false,
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
