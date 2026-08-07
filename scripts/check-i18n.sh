#!/usr/bin/env bash
# 校验中英双语词条。规则见 docs/rules/i18n.md §6。
#   · zh-CN 与 en-US 的 key 集合必须完全相同
#   · 代码里用到的 key 必须存在
#   · 词条文件里的 key 必须被用到
set -euo pipefail
cd "$(dirname "$0")/.."

DIR="frontend/src/i18n/locales"
ZH="$DIR/zh-CN.json"
EN="$DIR/en-US.json"

if [[ ! -f $ZH ]]; then
  echo "· 跳过 i18n 检查：$ZH 尚未创建"
  exit 0
fi

if ! command -v python3 >/dev/null; then
  echo "✗ 需要 python3" >&2; exit 1
fi

python3 - "$ZH" "$EN" <<'PY'
import json, re, sys, pathlib

zh_path, en_path = sys.argv[1], sys.argv[2]
fail = False

def load(p):
    try:
        return json.loads(pathlib.Path(p).read_text())
    except FileNotFoundError:
        print(f"✗ 缺少词条文件: {p}"); sys.exit(1)
    except json.JSONDecodeError as e:
        print(f"✗ {p} 不是合法 JSON: {e}"); sys.exit(1)

zh, en = load(zh_path), load(en_path)

def report(title, items, hint=""):
    global fail
    if not items: return
    fail = True
    print(f"✗ {title}:")
    for k in sorted(items)[:40]:
        print(f"    {k}")
    if len(items) > 40:
        print(f"    …还有 {len(items)-40} 条")
    if hint: print(f"  {hint}")
    print()

# ── 1. 两个语言文件的 key 必须一致 ────────────────────────────
report("en-US.json 缺少 zh-CN.json 里的 key", set(zh) - set(en),
       "两个语言文件永远同进同退，见 docs/rules/i18n.md §7")
report("zh-CN.json 缺少 en-US.json 里的 key", set(en) - set(zh))

# ── 2. 空词条 ────────────────────────────────────────────────
report("词条为空字符串", {k for k, v in {**zh, **en}.items() if not str(v).strip()},
       "不许留空占位")

# ── 3. 禁止中文原文当 key ────────────────────────────────────
report("key 里出现中文（禁止用中文原文当 key）",
       {k for k in zh if re.search(r'[一-龥]', k)})

# ── 4. 代码里用到但词条不存在 / 词条存在但没用到 ──────────────
src = pathlib.Path("frontend/src")
if src.exists():
    used, dynamic = set(), []
    for f in list(src.rglob("*.ts")) + list(src.rglob("*.tsx")):
        if "/i18n/" in str(f): continue
        text = f.read_text(errors="ignore")
        used |= set(re.findall(r"""\bt\(\s*['"]([a-zA-Z0-9_.]+)['"]""", text))
        # 动态拼 key：t('a.' + x) / t(`a.${x}`)
        if re.search(r"""\bt\(\s*(['"][^'"]*['"]\s*\+|`[^`]*\$\{)""", text):
            dynamic.append(str(f))

    report("代码里用到但词条不存在", used - set(zh))
    report("词条存在但代码里没用到", set(zh) - used, "删掉，或确认是动态引用后加注释豁免")
    report("动态拼接的 key（静态分析查不出缺失/未使用）", set(dynamic),
           "改成显式的 Record<T, TranslationKey> 映射，见 docs/rules/i18n.md §4")

if fail:
    print("修正方式见 docs/rules/i18n.md")
    sys.exit(1)
print(f"✓ i18n 词条一致（{len(zh)} 条 × 2 语言）")
PY
