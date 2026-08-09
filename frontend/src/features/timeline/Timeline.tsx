import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import styles from "./Timeline.module.css";
import { rendererFor, type TimelineEvent } from "./event-registry";
import { groupIntoTurns, initialsOf, mergeEvents, type Turn } from "./turns";

export type TimelineProps = {
  events: TimelineEvent[];
  /** 被关掉的事件类型；不传表示全显示。 */
  hidden?: ReadonlySet<string>;
};

/**
 * 时间线（验收点 V6）。
 *
 * ★ 渲染形态全部来自 `event-registry`，**这里没有一个 switch**
 * （`U2.3.2` 的 forbidden_changes 明写禁止）。加一类事件只加一条注册。
 */
export function Timeline({ events, hidden }: TimelineProps) {
  const { t } = useTranslation();

  const turns = useMemo(
    () => groupIntoTurns(mergeEvents(events, hidden)),
    [events, hidden],
  );

  if (turns.length === 0) {
    return <p className={styles.empty}>{t("timeline.empty")}</p>;
  }

  return (
    <div className={styles.list}>
      {turns.map((turn) => (
        <TurnBlock key={turn.key} turn={turn} />
      ))}
    </div>
  );
}

/** 一轮：头像 + 头部 + 说的话 + 干的活。 */
function TurnBlock({ turn }: { turn: Turn }) {
  const { t } = useTranslation();
  // ★★ 工具调用**默认收起**（照设计稿的可折叠抽屉）：用户要看的是
  // 「它说了什么」，而不是它跑过的每一条 grep。想看时点开。
  const [toolsOpen, setToolsOpen] = useState(false);

  if (turn.mine) {
    return (
      <div className={styles.row} data-align="end" data-turn-type={turn.firstType}>
        {/* ★ `data-event-type` 挂在**内容**上而不是这一轮上：
            两处同名的话，按类型数卡片会把「一轮」也数进去。 */}
        <div className={styles.mine} data-event-type={turn.firstType} data-shape="bubble">
          {turn.says.map((s) => s.text).join("")}
        </div>
      </div>
    );
  }

  return (
    <div
      className={styles.row}
      data-align="start"
      data-role={turn.role}
      data-turn-type={turn.firstType}
    >
      {/*
        ★★ 头像方块，照设计稿：`CL` / `CX`。
        一屏扫过去**不读文字**就分得清哪几条是同一个人说的。
      */}
      {turn.roleName !== "" && (
        <span
          className={styles.avatar}
          data-runtime={turn.runtime === "" ? undefined : turn.runtime}
          aria-hidden="true"
        >
          {initialsOf(turn.runtime)}
        </span>
      )}

      <div
        className={styles.body}
        data-speaker={turn.roleName === "" ? "app" : "agent"}
      >
        {turn.roleName !== "" && (
          <div className={styles.head}>
            <span className={styles.role} data-role={turn.role}>
              {turn.runtime !== "" && (
                <span className={styles.runtime}>{turn.runtime}</span>
              )}
              {turn.roleName}
            </span>
            {/*
              ★ 需求版本标签，形态照设计稿：`requirement v2 已冻结`。
              没有需求快照（0）时**整块不显示**——「v0」比不显示更糟。
            */}
            {turn.reqVersion > 0 && (
              <span className={styles.requirement} data-frozen={turn.reqFrozen}>
                {`requirement v${turn.reqVersion}`}
                {turn.reqFrozen && (
                  <span className={styles.frozen}>
                    {t("timeline.requirementFrozen")}
                  </span>
                )}
              </span>
            )}
          </div>
        )}

        {/* ★★ 说的话是**正文字号**，不是等宽小字——它是这一屏的主体。 */}
        {turn.says.map((s) => (
          <p
            key={s.key}
            className={styles.say}
            data-event-type={s.type}
            data-shape="bubble"
          >
            {s.text}
          </p>
        ))}

        {/*
          ★★ 干的活收在**一个抽屉**里，默认收起。
          十条 grep 各占一张卡片的话，用户要找的那句话被淹掉了。
        */}
        {turn.tools.length > 0 && (
          <div className={styles.tools}>
            <button
              type="button"
              className={styles.toolsToggle}
              onClick={() => setToolsOpen(!toolsOpen)}
              aria-expanded={toolsOpen}
            >
              {t(toolsOpen ? "timeline.hideTools" : "timeline.showTools", {
                count: turn.tools.length,
              })}
            </button>
            {toolsOpen && (
              <div className={styles.toolList}>
                {turn.tools.map((s) => (
                  <div
                    key={s.key}
                    className={styles.tool}
                    data-event-type={s.type}
                    data-shape="card"
                    data-status={s.status === "" ? undefined : s.status}
                  >
                    <span className={styles.toolKind}>
                      {t(rendererFor(s.type).labelKey)}
                    </span>
                    <span className={styles.toolDetail}>
                      {s.detail === "" ? s.text : s.detail}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {turn.lines.map((s) => (
          <div
            key={s.key}
            className={styles.line}
            data-event-type={s.type}
            data-shape="line"
          >
            <span className={styles.lineLabel}>
              {t(rendererFor(s.type).labelKey)}
            </span>
            {s.detail !== "" && (
              <span className={styles.lineDetail}>{s.detail}</span>
            )}
            {s.text !== "" && <span className={styles.lineDetail}>{s.text}</span>}
          </div>
        ))}
      </div>
    </div>
  );
}
