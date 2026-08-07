#!/usr/bin/env bash
# 校验里程碑规划的结构完整性。
#
# 为什么需要：里程碑是**另一个 AI 的施工图**。缺一个 forbidden_changes，
# 它就会顺手改掉不该改的；缺一条断言，验收标准就退化成一句愿望。
# 这类缺陷不会让任何别的检查变红——只会在几十轮之后表现为"代码乱了"。
#
# 校验：
#   ① 每个单元都有 goal / allowed_changes / forbidden_changes / stop_conditions
#   ② 每个单元都有验收标准表，且每条标准都带断言
#   ③ 单元编号唯一，且归属正确的里程碑（U2.x 只能在 M2）
#   ④ 子计划编号与单元编号自洽（U2.6.1 必须在 S2.6 之下）
#   ⑤ 每个里程碑都有目标 / 完成标志 / 全局停止条件
#   ⑦ roadmap 的「现在做」指针指向一个真实存在且尚未完成的单元
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

python3 - "$@" <<'PY'
import collections
import pathlib
import re
import sys

ROOT = pathlib.Path("docs/plan/milestones")
FIELDS = ["goal", "allowed_changes", "forbidden_changes", "stop_conditions"]
MILESTONE_SECTIONS = ["## 目标", "## 完成标志", "## 全局停止条件"]

problems = []
unit_ids = collections.Counter()
unit_status = {}                          # uid → ○ ◐ ✓ ⊘
unit_total = 0
crit_total = 0

files = sorted(p for p in ROOT.glob("M*.md"))
if not files:
    print("✗ 找不到任何里程碑文档（docs/plan/milestones/M*.md）")
    sys.exit(1)

for path in files:
    text = path.read_text(encoding="utf-8")
    milestone = path.name[:2]              # M0 / M1 ...
    digit = milestone[1]

    # ⑤ 里程碑级必备章节
    for sec in MILESTONE_SECTIONS:
        if sec not in text:
            problems.append(f"{path}: 缺章节「{sec.lstrip('# ')}」")

    # 按单元标题切块。标记：○ 未开始 ◐ 进行中 ✓ 已完成 ⊘ 被挡住
    parts = re.split(r"^### ([○◐✓⊘]) (U[0-9][0-9A-Za-z.]*)", text, flags=re.M)
    if len(parts) == 1:
        problems.append(f"{path}: 一个单元都没有（### <标记> U<编号>）")
        continue

    # 子计划标题，用于 ④
    subplans = set(re.findall(r"^## (S[0-9][0-9A-Za-z.]*)", text, flags=re.M))

    for i in range(1, len(parts), 3):
        mark, uid, body = parts[i], parts[i + 1], parts[i + 2]
        unit_status[uid] = mark
        unit_total += 1
        unit_ids[uid] += 1
        where = f"{path}:{uid}"

        # ③ 归属
        if not uid.startswith(f"U{digit}."):
            problems.append(f"{where}: 编号与里程碑不符（{milestone} 里应为 U{digit}.*）")

        # ④ 单元必须挂在一个真实存在的子计划下：U2.6.1 → S2.6
        m = re.match(r"U(\d+)\.(\w+)\.\d+$", uid)
        if m:
            want = f"S{m.group(1)}.{m.group(2)}"
            if want not in subplans:
                problems.append(f"{where}: 找不到对应的子计划 {want}")
        else:
            problems.append(f"{where}: 编号格式应为 U<里程碑>.<子计划>.<序号>")

        # ① 四要素
        missing = [f for f in FIELDS if f"`{f}`" not in body]
        if missing:
            problems.append(f"{where}: 缺 {' / '.join(missing)}")

        # ② 验收标准表 + 断言
        rows = re.findall(r"^\| (R\d+) \|(.+)$", body, flags=re.M)
        if not rows:
            problems.append(f"{where}: 没有验收标准表（| R1 | 标准 | 断言 |）")
            continue
        crit_total += len(rows)
        for rid, rest in rows:
            cells = [c.strip() for c in rest.split("|")]
            # 期望三列：标准 | 断言 |（末尾空）
            if len(cells) < 2 or not cells[1]:
                problems.append(f"{where} {rid}: 验收标准没有断言列——"
                                f"没有断言的标准是愿望不是标准")

dupes = [u for u, n in unit_ids.items() if n > 1]
for u in sorted(dupes):
    problems.append(f"单元编号重复 {u}（出现 {unit_ids[u]} 次）——编号只增不复用")

# ⑥ ★ 裁定里提到的单元必须真实存在
#
# 踩过一次：adr/0006 的 Q35/Q40 裁定「M1 新增 U1.7.3」「U4.7.1 → U1.8.4」，
# 但里程碑文档从没跟着改。下一个 AI 读了裁定去找这两个单元会扑空——
# 而在此之前，没有任何检查会报警。
#
# 只查「裁定/计划类文档提到 U<数字>.x.y」这种形式；正文里说「原 U4.7.1」
# 这类回溯引用用 `原 ` / `~~` 排除。
RULING_DIRS = [pathlib.Path("docs/adr"), pathlib.Path("docs/plan")]
UNIT_REF = re.compile(r"`?(U\d+\.\w+\.\d+)`?")

RETIRED_MARKERS = ("原 ", "原编号", "~~", "已挪", "废弃")

for d in RULING_DIRS:
    for path in sorted(d.rglob("*.md")):
        if path.parent.name == "milestones" and path.name.startswith("M"):
            continue                      # 里程碑自身在上面已经逐单元查过
        lines = path.read_text(encoding="utf-8").splitlines()

        # 文件级的「已废弃」声明：某个编号只要在**本文件任何一行**被标注为废弃，
        # 就整份文件放行它。
        #
        # ★ 为什么按文件而不是按行：ADR 是历史决策记录，**原文不许改**
        # （docs/adr 的规则），废弃说明只能补在下方。逐行判断会让
        # 「裁定原文那一行」永远红，逼着人去改历史。
        # 这条豁免不会放过真正的遗漏——遗漏的编号不会有人给它写废弃说明。
        retired = set()
        for line in lines:
            if any(k in line for k in RETIRED_MARKERS):
                retired.update(UNIT_REF.findall(line))

        for line in lines:
            if any(k in line for k in RETIRED_MARKERS):
                continue
            # 「`U4.7.1` → `U1.8.4`」的箭头左侧是被取代的旧编号，允许不存在；
            # 但箭头右侧必须存在，否则裁定就是空的。
            checked = re.sub(r"`?U\d+\.\w+\.\d+`?\s*(→|->)\s*", "", line)
            for uid in UNIT_REF.findall(checked):
                if uid not in unit_ids and uid not in retired:
                    problems.append(
                        f"{path}: 提到 {uid}，但里程碑里没有这个单元——"
                        f"裁定没有落进计划")

# ⑦ ★ roadmap 的「现在做」指针
#
# playbook §4.6 让接手的 AI「读 roadmap 找到下一个该做的单元」。
# 那一行一旦指向已完成的单元，下一轮 AI 要么重做一遍，要么得自己
# 翻六份里程碑文档重新找起点——这正是这份指针本来要省掉的事。
#
# 指针过期不会让任何别的检查变红，所以必须在这里守住。
ROADMAP = pathlib.Path("docs/plan/roadmap.md")
if ROADMAP.exists():
    text = ROADMAP.read_text(encoding="utf-8")
    m = re.search(r"\*\*现在做\*\*\s*\|\s*`(U[0-9][0-9A-Za-z.]*)`", text)
    if not m:
        problems.append(f"{ROADMAP}: 找不到「现在做」指针——"
                        f"接手的 AI 会不知道从哪开始（见 ai-playbook §4.6）")
    else:
        uid = m.group(1)
        if uid not in unit_status:
            problems.append(f"{ROADMAP}: 「现在做」指向 {uid}，但里程碑里没有这个单元")
        elif unit_status[uid] == "✓":
            problems.append(f"{ROADMAP}: 「现在做」指向 {uid}，但它已经是 ✓ 了——"
                            f"做完一个单元要回来改这一行")

if problems:
    print("✗ 里程碑规划有结构缺陷：")
    for p in problems:
        print(f"    {p}")
    print()
    print("  里程碑是另一个 AI 的施工图。缺 forbidden_changes 它就会改不该改的；")
    print("  缺断言，验收标准就退化成一句愿望。规则见 docs/plan/milestones/README.md")
    sys.exit(1)

print(f"✓ 里程碑规划完整："
      f"{len(files)} 章 · {unit_total} 个单元 · {crit_total} 条验收标准")
print("  每个单元都有 goal / allowed_changes / forbidden_changes / stop_conditions")
print("  每条验收标准都带断言")
PY
