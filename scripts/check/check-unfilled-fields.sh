#!/usr/bin/env bash
# 找「契约里有字段，而代码里从没填过它」。
#
# ★★ 这类缺陷最难发现：字段在、类型对、测试全绿，只是那个位置
# 永远是零值。用户看到的是一个空白，而没有任何地方报错。
#
# 已经撞到两次：
#   · skills 的 hit_count —— 契约里躺着，handler 从没填过（U10.4.1）
#   · resumable 的 unit_id —— 界面上那一行只剩一串 work-0N（本脚本立的）
set -euo pipefail
cd "$(dirname "$0")/../.."

python3 - <<'PYEOF'
import re, glob, sys

problems = []
files = {f: open(f, encoding='utf-8').read()
         for f in glob.glob('backend/internal/api/*.go') if not f.endswith('_test.go')}
# 跨文件搜：同一个包里，A 文件定义的类型常在 B 文件构造
whole = '\n'.join(files.values())

for f, s in files.items():
    for m in re.finditer(r'type (\w+(?:Body|Response)) struct \{(.*?)\n\}', s, flags=re.S):
        name, body = m.group(1), m.group(2)
        for fld in re.findall(r'^\t([A-Z]\w*)\s', body, flags=re.M):
            # 三种赋值形式都算填过了
            if re.search(r'\b' + fld + r':\s', whole):
                continue
            if re.search(r'\.' + fld + r'\s*=', whole):
                continue
            problems.append(f'{f}: {name}.{fld} 在契约里，但没有任何地方填它')

if problems:
    print('✗ 下列字段定义了却从没被赋值：')
    for p in problems:
        print(f'    {p}')
    print()
    print('  用户看到的是一个永远空白的位置，而没有任何地方报错。')
    print('  要么接上数据源，要么从契约里删掉——留着比没有更糟。')
    print('  确实是动态填充的话，在字段上加一行注释说明谁填它。')
    sys.exit(1)

print('✓ API 响应里的字段都有地方填')
PYEOF
