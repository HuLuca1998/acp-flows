package port

// SkillEntry 是扫到的一个 Skill（app 层看到的样子）。
//
// ★ 与 `fsstore/skill.Entry` 分开定义：那边是「盘上有什么」，
// 这边是「界面要什么」。合成一个的话，`api` 就得 import `fsstore`，
// 而 port 存在的意义正是让上层不认识具体实现。
type SkillEntry struct {
	Name             string
	Dir              string
	Version          string
	Description      string
	Compatibility    string
	Scope            string
	Source           string
	Status           string
	ValidationOK     bool
	ValidationReason string
}

// SkillScanner 扫出可用的 Skill。
//
// ★ **只读**。这一层往下的实现绝不改用户的文件（红线 3）——
// 扫描类函数要有「扫完全目录内容哈希不变」的测试守着。
type SkillScanner interface {
	// ScanGlobal 扫全局库（`~/.acpflows/skills`）。
	//
	// ★ 目录不存在返回**空列表而不是错误**：绝大多数用户一开始
	// 就没有这个目录，当成错误的话设置页会显示一条吓人的报错。
	ScanGlobal() ([]SkillEntry, error)

	// DiscoverInProject 找出项目里已有的 skill（`**/skills`，跳过重目录）。
	//
	// ★ 创建项目时用它回答「发现已有 Skill 目录 · N」。
	// 每条都带**项目内的相对路径**当来源——用户要能照着去找。
	DiscoverInProject(projectPath string) ([]SkillEntry, error)
}
