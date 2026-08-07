# AGENTS.md · frontend/src/features/timeline

时间线：把事件流渲染成用户看得懂的东西。
仓库总纲见根 [`AGENTS.md`](../../../../AGENTS.md)。

---

## 一条硬规则：注册表，不是 switch

**加一类事件只加一条注册**（`event-registry.ts` 的 `RENDERERS`），
不改任何既有代码。`U2.3.2` 的 forbidden_changes 明写禁止 `switch` 分发。

`EVENT_KINDS` 由注册表推导，**不另外维护一份列表**——两份必然漂移。

事件类型的真源是 [`api/openapi.yaml`](../../../../api/openapi.yaml) 的 `Event.type` 枚举。
`event-registry.test.ts` 拿它逐条比对：少一条会红，多一条也会红。

## 未知类型必须有兜底

后端加一类事件而前端还没跟上时，用户看到的应该是
「有一条我暂时看不懂的记录」，**而不是白屏**——白屏会让他以为整个应用坏了。

`rendererFor` 永远返回渲染器，不返回 `undefined`。

> ★ 测试要断言「**不是兜底**」而不是「defined」。只看 defined 的话，
> 漏注册一类事件时兜底会接住它，断言照样绿。第一版就是这么写的，造负例才发现。

## 什么该合并，什么不该

**只有文本流合并**（`merge: true`）。流式文本一个字一个字地来，
每片一个气泡的话界面会在打字过程中疯狂重排。

**工具调用不合并**——两次就是两次。合并的话用户会以为 AI 只动了一个文件。

## 文案全部走 i18n

过滤器的分组标题与条目文案**逐字照 `design/ACP Duet 1a.dc.html`
的「时间线显示」面板**。对不上的话，用户在设计稿截图与实际界面之间
对照会以为自己点错了地方。

硬编码文案是 forbidden_changes，`make check-i18n` 会拦。
