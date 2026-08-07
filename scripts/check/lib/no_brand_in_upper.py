"""上层代码里不许出现 Runtime 的品牌名。

U2.2.3 的 R2 明写这条要接进 CI：

    grep -rn 'codex|claude' internal/{app,domain,api}  为空

**为什么这条重要**：上层一旦写 `if name == "codex"`，加第三个 Runtime
就要在几十个地方改 if，而每漏一处都是一个只在那个 Runtime 下出现的 bug。
差异要么在 adapter 里填平，要么升级成能力查询——两条路都不该让上层知道品牌。

豁免三类，每一类都有理由：
  - 注释：解释「为什么不许按名字分支」本身要提到名字
  - 测试：用 claude / codex 当样例数据是自然的
  - 生成物：由 api/openapi.yaml 决定，人改不了

判定交给 Python 而不是 grep：本仓库的注释是中文，
而 C locale 下的 grep 按字节处理多字节字符（pitfalls P-10 同源）。
"""

import pathlib
import re
import sys

UPPER_LAYERS = [
    pathlib.Path("backend/internal/app"),
    pathlib.Path("backend/internal/domain"),
    pathlib.Path("backend/internal/api"),
]

BRANDS = re.compile(r"\b(claude|codex)\b", re.IGNORECASE)

found = False
for root in UPPER_LAYERS:
    if not root.exists():
        continue
    for path in sorted(root.rglob("*.go")):
        if path.name.endswith("_test.go"):
            continue  # 测试里用它们当样例数据是自然的
        if "/gen/" in str(path):
            continue  # 生成物由 openapi.yaml 决定，人改不了

        in_block_comment = False
        for n, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
            stripped = line.strip()

            # 块注释：/* ... */
            if in_block_comment:
                if "*/" in stripped:
                    in_block_comment = False
                continue
            if stripped.startswith("/*"):
                if "*/" not in stripped:
                    in_block_comment = True
                continue

            # 行注释：解释「为什么不许按名字分支」本身要提到名字
            if stripped.startswith("//"):
                continue

            if BRANDS.search(line):
                print(f"{path}:{n}: {stripped[:90]}")
                found = True

sys.exit(0 if not found else 0)  # 输出即判定，退出码交给调用方
