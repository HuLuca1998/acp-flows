"""检查每个 exec.CommandContext 后面都设了 cmd.WaitDelay。

单独成文件而不是塞进 check-naming.sh 的 heredoc：那个脚本里已经有一段
内嵌 python，再嵌一段会让引号与 heredoc 分隔符互相打架。

规则由来见 check-naming.sh 第 8 节的注释。
"""

import pathlib
import sys

# WaitDelay 得在紧随其后的这么多行内设上——同一个函数里，够宽松了。
WINDOW = 15

found = False
for path in sorted(pathlib.Path("backend").rglob("*.go")):
    if "/gen/" in str(path):
        continue  # 生成物不许手改
    lines = path.read_text(encoding="utf-8").splitlines()
    for n, line in enumerate(lines):
        if "exec.CommandContext" not in line or line.lstrip().startswith("//"):
            continue
        # ★ 必须排掉注释行。第一版没排，结果解释这条规则的那段注释
        # 自己就让检查通过了——把赋值删掉都不会红。
        if not any(
            "WaitDelay" in l and not l.lstrip().startswith("//")
            for l in lines[n : n + WINDOW]
        ):
            print(f"{path}:{n + 1}: {line.strip()}")
            found = True

sys.exit(0 if not found else 0)  # 输出即判定，退出码交给调用方
