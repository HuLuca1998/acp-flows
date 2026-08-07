# 禁止清单（完整版）

> 从根 `AGENTS.md` 下沉。L0 只留最高频的几条，完整清单在这里。
> **出现即视为不合规，改动回滚。**

出现即视为不合规，改动回滚。

**设计与组织**（完整清单见 `docs/design-principles.md` §8）

- ✗ **品牌判断离开 adapter** —— `grep -rn 'codex\|claude' backend/internal/{app,domain,api}` 必须是空的。
  claude 与 codex 的差异一律在 `acp/adapter/` 内部填平，上层只表达意图与查询能力
- ✗ 接口方法名照抄协议方法名（`SetMode` → 应该是 `RestrictPermissions`）—— 协议一变全仓库跟着改
- ✗ 复制粘贴复用 —— 想复制时停下来，用嵌入 / 模板方法 / 装饰器
- ✗ `switch` 分发同类实现 —— 用注册表，加第 3 个实现不该改老代码
- ✗ 包内平铺超过 10 个文件；按技术种类切文件（`models.go` `utils.go` `handlers.go`）
- ✗ 接口和它唯一的实现放同一个包（接口定义在**使用方**）
- ✗ 上帝接口；只有一个实现就抽接口（过早抽象）
- ✗ 单文件 > 400 行、单函数 > 60 行

**工程**

- ✗ 先写实现后补测试
- ✗ 断言恒真、mock 喂 mock、只运行不断言的测试
- ✗ 改了接口不改 `api/openapi.yaml`
- ✗ `domain` 包 import 任何基础设施包或做 IO
- ✗ **领域模型挂 `gorm` 标签** —— 实体在 `store/entity/`，中间隔一个 `mapper/`
- ✗ GORM 的 `Updates` 传 struct（零值静默丢更新）、用 `Find` 查单条、用 `Save`
- ✗ 用 `AutoMigrate` 代替版本化迁移；不开 `foreign_keys` pragma；事务里做 IO
- ✗ 测试读写 `~/.acpflows`、用户真实仓库或真实令牌
- ✗ 未经批准新增第三方依赖
- ✗ 把 Agent 的转述当证据（证据必须由应用直接采集）
- ✗ 提交信息里写没有命令输出支撑的结论

**国际化**（完整清单见 `docs/i18n.md` §8）

- ✗ 组件里硬编码用户可见文本 —— 一律 `t('key')`
- ✗ 翻译状态词 / 标识符 / 命令 / 路径 / ID —— 它们在中英两版里都保持英文等宽
- ✗ 只更新 `zh-CN.json` 不更新 `en-US.json` —— 两个语言文件永远同进同退
- ✗ 后端返回中文文案给界面展示 —— 后端只返回错误码，前端负责翻译

**设计**（完整清单见 `design/Duet Spec.dc.html` 第 10 节）

- ✗ 写死 hex 或新造颜色（含语义色，语义色只有 `--color-pass` / `--color-fail` 两个）
- ✗ emoji、Unicode 几何符号当图标，或自绘 SVG 图标（图标只从 Phosphor regular 取）
- ✗ 实心填充的主按钮、彩色渐变按钮
- ✗ 悬浮层影响正文布局，或被父级 `overflow:hidden` 裁切
- ✗ 同类选择器在不同页面位置不一致
- ✗ 纯图标按钮没有中文 tooltip
- ✗ 设计 ACP 不支持的设置项（如按角色设模型——模型不在协议里）
- ✗ 用弹窗打断执行中的单元来展示非阻塞信息
- ✗ 结论不带证据入口
- ✗ 自动把聊天原文或一次成功经验写成长期记忆

---
