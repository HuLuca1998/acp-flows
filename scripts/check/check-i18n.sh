#!/usr/bin/env bash
# 校验中英双语词条。规则见 docs/rules/i18n.md §6。
#   · zh-CN 与 en-US 的 key 集合必须完全相同
#   · 代码里用到的 key 必须存在
#   · 词条文件里的 key 必须被用到
set -euo pipefail
cd "$(dirname "$0")/../.."

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

# ★ 先跑 key 提取器自己的负例集：一条抓不到东西的检查比没有检查更糟。
python3 "$(dirname "$0")/lib/i18n_keys_test.py" || exit 1

python3 - "$ZH" "$EN" "$(dirname "$0")/lib" <<'PY'
import json, re, sys, pathlib

# key 提取逻辑在 lib/i18n_keys.py，它有自己的负例集（i18n_keys_test.py）。
# 放在那儿是为了能对检查本身造负例——原来内嵌在这里的正则漏过一次。
sys.path.insert(0, sys.argv[3])
import i18n_keys

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
    used, required, dynamic = set(), set(), []
    for f in list(src.rglob("*.ts")) + list(src.rglob("*.tsx")):
        if "/i18n/" in str(f): continue
        # 注释的剥离在 lib/i18n_keys.py 里做——注释里的反例示例
        # （t(`error.${code}`) 这种教学用法）不是违规。
        raw = f.read_text(errors="ignore")
        # ★ 两个方向要**分开判定**，合成一个会丢掉其中一个：
        #
        # ① t(...) 里的点分字面量**一定**是 i18n key —— 缺词条就该红。
        #    取的是整个实参，不是「紧跟在 t( 后面的那个」：
        #    t(x?.key ?? 'a.b') 与 t(c ? 'a.b' : 'c.d') 都要算上。
        #    只认紧跟形式的旧正则漏过一次，代价是 `page.chat.title`
        #    七个字直接显示在界面上（lib/i18n_keys.py 的文件头有始末）。
        required |= i18n_keys.required_keys(raw)
        #
        # ② 其余点分字面量（hintKey="x" / 注册表 titleKey / 错误码映射的值）
        #    只用来标记「这个 key 有人用」。它们无法与普通字符串区分，
        #    所以不能反过来要求「必须存在于词条表」——那会把
        #    'application/json' 之类的字符串误判成缺失的词条。
        used |= i18n_keys.mentioned_keys(raw) & set(zh)
        if i18n_keys.has_dynamic_key(raw):
            dynamic.append(str(f))

    report("代码里用到但词条不存在", required - set(zh))
    report("词条存在但代码里没用到", set(zh) - used - required, "删掉，或确认是动态引用后加注释豁免")
    report("动态拼接的 key（静态分析查不出缺失/未使用）", set(dynamic),
           "改成显式的 Record<T, TranslationKey> 映射，见 docs/rules/i18n.md §4")

if fail:
    print("修正方式见 docs/rules/i18n.md")
    sys.exit(1)
print(f"✓ i18n 词条一致（{len(zh)} 条 × 2 语言）")
PY
