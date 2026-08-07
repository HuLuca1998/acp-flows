package runtime

// 注册表。**加第三个 Runtime 只在这里加一条**，不改 Detect 一行，
// 不在任何地方写 `if name == "..."`（design-principles §4.4）。
//
// ★ 这里的每条命令都是 2026-08-07 在真机上实测过的，不是照着文档抄的。
// 改动前先自己跑一遍——CLI 的子命令改起来很随意，而猜错的表现是
// 「用户明明装好了，界面说没装」。

// Registered 返回内置的 Runtime 规格。
func Registered() []Spec {
	return []Spec{
		{
			Name: "claude",
			// ★ 适配器与 CLI 是**两个不同的可执行文件**。
			// 能不能跑取决于适配器；登没登录归 CLI 管。
			Bin:         "claude-agent-acp",
			VersionArgs: []string{"--version"}, // 实测输出：0.63.0
			AuthBin:     "claude",
			AuthArgs:    []string{"auth", "status"},
			// 实测 `claude auth status` **未登录时也返回 exit 0**，
			// 结论在 JSON 的 loggedIn 字段里。只看退出码会把没登录报成就绪。
			AuthOKSubstring: `"loggedIn": true`,
			// acp-field-notes §5 坑 1：带着这些标记，claude-agent-acp
			// 会误判自己跑在另一个 agent 内部而拒绝服务。
			EnvRemove:      []string{"CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SSE_PORT"},
			InstallCommand: "npm i -g @agentclientprotocol/claude-agent-acp",
			LoginCommand:   "claude auth login",
		},
		{
			Name:        "codex",
			Bin:         "codex-acp",
			VersionArgs: []string{"--version"}, // 实测输出：@agentclientprotocol/codex-acp 1.1.7
			AuthBin:     "codex",
			AuthArgs:    []string{"login", "status"},
			// 实测已登录时 stderr 上是「Logged in using an API key - ***」。
			// 注意是 **stderr**，只读 stdout 会永远匹配不上。
			AuthOKSubstring: "Logged in",
			EnvRemove:       []string{"CODEX_SANDBOX", "CODEX_SANDBOX_NETWORK_DISABLED"},
			InstallCommand:  "npm i -g @agentclientprotocol/codex-acp",
			LoginCommand:    "codex login",
		},
	}
}
