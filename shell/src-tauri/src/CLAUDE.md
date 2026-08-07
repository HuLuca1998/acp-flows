# CLAUDE.md · shell/src-tauri/src

本目录的规则见同目录 [`AGENTS.md`](AGENTS.md)，仓库总纲见根 [`AGENTS.md`](/AGENTS.md)。

**不要在本文件里复制 `AGENTS.md` 的内容**——重复的内容必然漂移。

## 本目录必用的 skill

- 开始一个开发单元：`tdd-unit`

## 这一层没法用单元测试验

sidecar 拉起、托盘图标、窗口关闭行为**都要打一个真包出来点**。
`cargo test` 跑绿不代表这些能用——改完必须按 `AGENTS.md` 的命令打包，
然后亲手验证：窗口能开、菜单栏有图标、点红叉不退出、菜单里能强制退出。

**不要因为「编译过了」就说做完了。**
