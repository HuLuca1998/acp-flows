package work

import "github.com/HuLuca1998/acp-flows/backend/internal/constant"

// View 是交给上层的工作视图。
type View struct {
	ID       string
	State    constant.WorkState
	Project  string
	Worktree string
	Prompt   string
	// Title 是列表里显示的名字，取自用户提的那句需求。
	Title string
	// Branch 与 BaseCommit 是这个工作的 git 现场。
	//
	// ★ 右栏「领先几个 commit」与验收时的 diff 都要 BaseCommit 当起点，
	// 不记的话「这个工作到底改了什么」没有答案。
	Branch     string
	BaseCommit string
}
