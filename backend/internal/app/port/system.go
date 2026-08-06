// Package port 定义 app 层对外部世界的全部抽象。
//
// 硬规则：这里只有 interface 和它们用到的领域类型，零 struct 实现、
// 零第三方库 import。基础设施包反过来实现这些接口（Go 是结构化类型，
// 它们甚至不需要 import 本包）。
//
// 接口要小。一个用例只依赖它真正需要的两三个方法——巨型 fake 是假测试的温床。
package port

import "time"

// Clock 是时间源。
//
// domain 与 app 层禁止直接调 time.Now()：那会让测试变成薛定谔的。
// 生产实现在 platform，测试实现在 tests/testutil。
type Clock interface {
	// Now 返回当前时间。生产实现返回 UTC。
	Now() time.Time
}

// IDGen 生成带类型前缀的标识符。
//
// 主键是字符串而非自增整数——界面上大量展示这些 ID 且要求等宽显示，
// 用整数会让前端到处拼字符串。理由见 docs/adr/0005-persistence.md。
type IDGen interface {
	// NextID 返回形如 "work-08" 的标识符。prefix 不含连字符。
	NextID(prefix string) string
	// NextULID 返回按时间可排序的标识符，用于高频写入的事件表。
	NextULID() string
}

// Paths 是数据目录下全部路径的唯一来源。
//
// ★ 任何地方直接拼 os.UserHomeDir() + "/.acpflows" 都是违规（铁律 6）。
// 测试里由 testutil 重定向到 t.TempDir()，隔离守卫会拦住漏网的直接访问。
type Paths interface {
	// DataDir 是全局数据目录，生产环境为 ~/.acpflows。
	DataDir() string
	// DBPath 是 SQLite 数据库文件路径。
	DBPath() string
	// RuntimeSession 是 duetd 写端口与 token 的文件（权限 0600）。
	RuntimeSession() string
	// RuntimesDir 存放多版本并存的 ACP Runtime。
	RuntimesDir() string
	// CredentialsPath 是加密后的 GitHub 令牌存放位置。
	//
	// 绝不写入任何项目目录，绝不进入 Agent 上下文。
	CredentialsPath() string
	// WorktreeRoot 是各个 Work 的 git worktree 根目录。
	WorktreeRoot() string
}
