package port

import "context"

// RuntimeStatus 是一个 ACP Runtime 的检测结论。
//
// ★ 上层按 Status 分支，**绝不按 Name 分支**。「codex 没登录该敲什么」
// 这类知识只存在于检测实现的注册表里，是数据不是代码——加第三个 Runtime
// 不该让任何一处 if 变长（docs/rules/design-principles.md §4.4）。
type RuntimeStatus struct {
	// Name 是 Runtime 标识，如 claude / codex。
	Name string
	// Status 取值：ready / not_installed / not_authenticated / probe_failed。
	//
	// probe_failed 不能省。没有它，「检测本身失败了」只能并进 not_installed，
	// 界面就会对着一个装好的 Runtime 说「请先安装」。
	Status string
	// Version 是探到的版本原文，探不到时为空。
	Version string
	// Path 是可执行文件位置，供排查用。
	Path string
	// Remedy 是用户可以直接复制去终端敲的一整条命令。
	// ready 时为空；probe_failed 时也为空——那时没法确定要修什么。
	Remedy string
	// Detail 是失败原因原文，给开发者排查，不直接展示给用户。
	Detail string
}

// RuntimeDetector 查本机装了哪些 ACP Runtime、能不能用。
//
// 实现必须做到：**零模型开销**（只问版本和登录态，绝不发起会话），
// 且一个 Runtime 检测失败不影响其余的结论。
type RuntimeDetector interface {
	DetectAll(ctx context.Context) []RuntimeStatus
}
