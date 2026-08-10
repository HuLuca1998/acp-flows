#!/usr/bin/env bash
# 找「后端有端点，前端够不着」。
#
# ★★ 探索时撞到过一个功能级缺口：检查点、恢复逻辑、`GET /v1/system/resume`
# 后端全做完了，而界面上没有任何入口——用户永远看不到
# 「有 2 个工作可以接着做」，那整套代码等于没用。
#
# ★ 这不是硬错误：有些端点本来就只给 CLI / 别的服务用。
# 所以只报告，让人自己判断——但**必须看见**。
set -euo pipefail
cd "$(dirname "$0")/../.."

python3 - <<'PYEOF'
import re, glob, sys

routes = set()
for f in glob.glob('backend/internal/api/*.go'):
    if f.endswith('_test.go'):
        continue
    s = open(f, encoding='utf-8').read()
    for m in re.finditer(r'mux\.HandleFunc\("(\w+) (/v1/[^"]+)"', s):
        routes.add((m.group(1), m.group(2)))

# ★★ **排除测试文件**：契约测试里会把每个路径都列一遍，
# 不排的话这个检查永远是绿的——它自己就成了一条假绿的防线。
# 第一版就栽在这儿：resume 明明没有界面入口，脚本却说「都够得着」。
front = '\n'.join(open(f, encoding='utf-8').read()
                  for f in glob.glob('frontend/src/**/*.ts*', recursive=True)
                  if '/gen/' not in f and '.test.' not in f)

unreached = []
for method, path in sorted(routes):
    # 前端用 openapi-fetch，路径里不带 /v1 前缀，且参数是 {id} 形式
    probe = path.replace('/v1', '', 1)
    probe = re.sub(r'\{\w+\}', '{', probe)
    head = probe.split('{')[0].rstrip('/')
    if head and head not in front:
        unreached.append(f'{method} {path}')

if unreached:
    print('· 下列端点前端够不着（可能是功能级缺口，也可能本来就不给界面用）：')
    for u in unreached:
        print(f'    {u}')
    print()
    print('  后端做完而界面没入口的话，那整套代码等于没用——')
    print('  用户看不到它，也就不会知道自己少了什么。')
else:
    print('✓ 每个端点前端都够得着')
PYEOF
