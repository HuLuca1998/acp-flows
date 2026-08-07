# AGENTS.md · backend/internal/acp/adapter

把不同 ACP Runtime 的差异**内化掉**。仓库总纲见根 [`AGENTS.md`](../../../../AGENTS.md)。

---

## 这个包存在的唯一理由

**上层只表达意图，不认识「这是 Claude 还是 Codex」。**

上层一旦写 `if name == "codex"`，加第三个 Runtime 就要在几十个地方改 if，
而**每漏一处都是一个只在那个 Runtime 下出现的 bug**——那种 bug 最难发现，
因为开发机上装的往往只有一个。

`scripts/check/lib/no_brand_in_upper.py` 把这条接进了 CI：
`app` / `domain` / `api` 的非注释、非测试、非生成代码里出现品牌名即红。

## 差异只有两条出路

| 出路 | 什么时候用 | 例子 |
|---|---|---|
| **在这里填平** | 两端语义相同、只是形状不同 | 按 `category` 取配置项而不是按 `id` |
| **升级成能力查询** | 两端真的不一样 | 探针跑一遍，上层问「支不支持 X」 |

**两条路都不需要上层知道品牌。** 哪条都走不通时才停下来找人
（`U2.2.3` 的 stop_conditions）。

## 按 category 取，不按 id 取

推理强度在 claude 是 `effort`、在 codex 是 `reasoning_effort`，
但**两端 category 都是 `thought_level`**。

按 id 取就要维护一张映射表，每加一个 Runtime 加一行——那正是「差异没内化」。
协议本身给了语义层的稳定键（`acp-field-notes.md` §7.1），用它就好。

> ★ 用 `CategoryOrEmpty()` 而不是直接解引用：
> **claude 的 agent 选项 category 是空字符串，codex 的某些选项根本没这个字段**。
> 直接解引用会在其中一端 panic，而崩的是用户打开设置页的时候。

## 没探过的能力一律当不支持

乐观假设的代价是上层走进一条走不通的路，错误在很远的地方才浮现；
保守判断的代价只是少用一个可能可用的特性。**两者不对等。**

## 测试怎么写

**同一批断言跑遍两端，测试代码里不许出现 `if impl ==`。**
有的话就说明差异没被内化——只是从生产代码搬进了测试代码。

样例数据用 `acp-field-notes.md` §7.1 里**实测记下的真实形态**，
不要自己编一个「看起来合理」的——编出来的往往正好避开了真实的坑。
