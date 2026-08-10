package project_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/fsstore/project"
)

// M3 U3.1.1 · 初始化预演与执行
//
// ★★ 这一族守的是**用户点确认之前就知道我们要动什么**。
// 他交出来的是自己的代码仓库——「预演里没说的事被做了」是最不该发生的。

// fingerprint 算整棵目录树的指纹：路径 + 内容 + 是不是目录。
func fingerprint(t *testing.T, root string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		h.Write([]byte(rel))
		if d.IsDir() {
			h.Write([]byte("/dir"))
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// gitRepo 造一个看起来像 git 仓库的目录。
func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func planFor(t *testing.T, root string) *project.Plan {
	t.Helper()
	p, err := project.MakePlan(root)
	if err != nil {
		t.Fatalf("算计划: %v", err)
	}
	return p
}

// ★★ R1 · 预演一个字节都不写。
//
// 判据是**全目录指纹**（路径 + 内容 + 目录结构），
// 不是「我们没调 os.WriteFile」——前者才管得住「顺手先建个目录」。
func TestMakePlan_R1_WritesNothing(t *testing.T) {
	root := gitRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := fingerprint(t, root)
	p := planFor(t, root)
	after := fingerprint(t, root)

	if before != after {
		t.Errorf("算个计划就动了用户的目录：\n之前 %s\n之后 %s", before, after)
	}
	if len(p.Actions) == 0 {
		t.Error("计划是空的——那用户什么都看不到")
	}
	// ★ 计划本身要说得出每一步为什么
	for _, a := range p.Actions {
		if a.Reason == "" {
			t.Errorf("%s 这一步没写为什么——用户凭什么点确认", a.Path)
		}
	}
}

// ★★ R2 · 预演说的和执行做的**逐条一致**。
//
// 判据：同一份 Plan 分别取「说要动的路径」和「执行后真的出现的路径」，
// 两个集合必须相等。分别写两套代码的话它们必然漂移，
// 而漂移的方向永远是「预演里没说的那件事被做了」。
func TestApply_R2_DoesExactlyWhatThePlanSaid(t *testing.T) {
	root := gitRepo(t)
	p := planFor(t, root)

	said := map[string]bool{}
	for _, a := range p.Pending() {
		said[a.Path] = true
	}

	before := snapshotPaths(t, root)
	if err := project.Apply(p); err != nil {
		t.Fatalf("执行: %v", err)
	}
	after := snapshotPaths(t, root)

	// 真的新出现的路径
	appeared := map[string]bool{}
	for path := range after {
		if !before[path] {
			appeared[path] = true
		}
	}

	for path := range appeared {
		if !said[path] {
			t.Errorf("★ 建了计划里没说的东西：%s", path)
		}
	}
	for path := range said {
		// 追加类动作不会新建路径（文件本来就在），单独排除
		if !appeared[path] && !fileExists(path) {
			t.Errorf("★ 计划里说要建 %s，但它没出现", path)
		}
	}
}

// ★★ R3 · `.gitignore` 是**追加**不是覆盖。
//
// 覆盖掉等于把用户自己的规则删了，而他不会立刻发现。
func TestApply_R3_AppendsToGitignoreKeepingEveryLine(t *testing.T) {
	root := gitRepo(t)
	original := "node_modules/\ndist/\n*.log\n"
	gitignore := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(gitignore, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := project.Apply(planFor(t, root)); err != nil {
		t.Fatalf("执行: %v", err)
	}

	got := readFile(t, gitignore)
	for _, line := range []string{"node_modules/", "dist/", "*.log"} {
		if !strings.Contains(got, line) {
			t.Errorf("用户原有的规则 %q 没了——覆盖掉他的 .gitignore 是不可逆的", line)
		}
	}
	if !strings.Contains(got, ".acpflows/runs/") {
		t.Error("没追加忽略规则")
	}
}

// ★ 跑第二次不重复追加。
//
// 重复的话，`.gitignore` 会随着每次重新初始化越长越长。
func TestApply_R3b_IsIdempotent(t *testing.T) {
	root := gitRepo(t)

	for range 3 {
		if err := project.Apply(planFor(t, root)); err != nil {
			t.Fatalf("执行: %v", err)
		}
	}

	got := readFile(t, filepath.Join(root, ".gitignore"))
	if n := strings.Count(got, ".acpflows/runs/"); n != 1 {
		t.Errorf("忽略规则出现了 %d 次——重复初始化会让 .gitignore 越长越长", n)
	}
}

// ★ 用户的 .gitignore 最后一行没有换行符时，别和他的规则粘成一行。
//
// 粘上的话那条规则就此失效，而 git 不会报错。
func TestApply_R3c_HandlesMissingTrailingNewline(t *testing.T) {
	root := gitRepo(t)
	gitignore := filepath.Join(root, ".gitignore")
	// 注意：结尾**没有**换行
	if err := os.WriteFile(gitignore, []byte("dist/"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := project.Apply(planFor(t, root)); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, gitignore)
	if strings.Contains(got, "dist/.acpflows") {
		t.Errorf("和用户的最后一条规则粘成一行了：%q——那条规则就此失效而 git 不会报错", got)
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if lines[0] != "dist/" {
		t.Errorf("第一行 = %q", lines[0])
	}
}

// ★ 被注释掉的同名规则不算数。
//
// 用 `strings.Contains` 判断的话，`#.acpflows/runs/` 会让我们以为
// 规则已经生效，而实际它被注释着。
func TestApply_R3d_CommentedRuleDoesNotCount(t *testing.T) {
	root := gitRepo(t)
	gitignore := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(gitignore, []byte("# .acpflows/runs/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := project.Apply(planFor(t, root)); err != nil {
		t.Fatal(err)
	}

	got := readFile(t, gitignore)
	var active int
	for _, l := range strings.Split(got, "\n") {
		if strings.TrimSpace(l) == ".acpflows/runs/" {
			active++
		}
	}
	if active != 1 {
		t.Errorf("生效的忽略规则有 %d 条——被注释掉的那条不该算数：\n%s", active, got)
	}
}

// ★★ R4 · 不是 git 仓库时如实报告，**且不擅自 git init**。
//
// 在别人的目录里建一个 git 仓库是不可逆的，而他可能有自己的打算。
func TestMakePlan_R4_NonRepoIsReportedNotInitialized(t *testing.T) {
	root := t.TempDir() // 没有 .git

	p := planFor(t, root)
	if p.IsGitRepo {
		t.Error("把一个非仓库报成了仓库")
	}

	if err := project.Apply(p); err != nil {
		t.Fatalf("执行: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		t.Error("★ 擅自跑了 git init——在别人的目录里建仓库是不可逆的")
	}
	// 非仓库也不该凭空造一个 .gitignore
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); err == nil {
		t.Error("不是 git 仓库却建了 .gitignore")
	}
	// 但 .acpflows 该建
	if _, err := os.Stat(filepath.Join(root, project.DuetDirName)); err != nil {
		t.Errorf("非仓库也该能初始化：%v", err)
	}
}

// ★★ R5 · 中途失败**不留半成品**。
//
// 判据是**我们自己建的东西回到原样**：留下一个残缺的 `.acpflows/` 的话，
// 用户既不知道它在那儿，也不知道它是不是完整的；而重试时那个残缺目录
// 还会让「已经在了」的判断出错。
//
// ★★ **`.gitignore` 里追加的那一行不回滚**，这是有意的：
// 撤销它要读-改-写用户自己的文件，而这中间他可能已经改过——
// 一次失败的初始化毁掉他手写的规则，比留下一行无害的忽略规则糟得多。
// 那一行指向一个还不存在的目录，本身没有任何作用，
// 而且下次初始化成功时正好用得上（`hasIgnoreLine` 保证不会重复追加）。
func TestApply_R5_RollsBackOnFailure(t *testing.T) {
	root := gitRepo(t)
	p := planFor(t, root)

	// 注入一个必然失败的动作：往一个**已经是目录**的路径写文件。
	// ★ 这个 blocker 目录代表「用户自己已经有的东西」——
	// 回滚绝不能把它连带删掉（`MkdirAll` 对已存在目录成功返回，
	// 记成「这次创建的」再 RemoveAll 就会毁掉它）。
	blocker := filepath.Join(root, project.DuetDirName, "blocked")
	if err := os.MkdirAll(blocker, 0o755); err != nil {
		t.Fatal(err)
	}
	duetBefore := fingerprint(t, filepath.Join(root, project.DuetDirName))

	p.Actions = append(p.Actions, project.Action{
		Kind:   project.ActionCreateFile,
		Path:   blocker, // 是目录，写文件必失败
		Lines:  []string{"x"},
		Reason: "注入的失败",
	})

	if err := project.Apply(p); err == nil {
		t.Fatal("注入的失败没被报出来")
	}

	if got := fingerprint(t, filepath.Join(root, project.DuetDirName)); got != duetBefore {
		t.Errorf("失败之后 .acpflows 里留下了半成品：\n之前 %s\n之后 %s\n目录树：\n%s",
			duetBefore, got, treeOf(t, root))
	}
	// 用户已有的东西必须还在
	if _, err := os.Stat(blocker); err != nil {
		t.Errorf("★ 回滚把用户已有的目录删了：%v", err)
	}
}

// ★ 回滚**不删用户自己的文件**。
//
// 删掉的话，一次失败的初始化会顺手清掉他的 .gitignore。
func TestApply_R5b_RollbackKeepsUserFiles(t *testing.T) {
	root := gitRepo(t)
	gitignore := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(gitignore, []byte("node_modules/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p := planFor(t, root)
	blocker := filepath.Join(root, project.DuetDirName, "blocked")
	if err := os.MkdirAll(blocker, 0o755); err != nil {
		t.Fatal(err)
	}
	p.Actions = append(p.Actions, project.Action{
		Kind: project.ActionCreateFile, Path: blocker, Lines: []string{"x"}, Reason: "注入的失败",
	})

	if err := project.Apply(p); err == nil {
		t.Fatal("注入的失败没被报出来")
	}

	if _, err := os.Stat(gitignore); err != nil {
		t.Errorf("★ 回滚把用户的 .gitignore 删了：%v", err)
	}
	if got := readFile(t, gitignore); !strings.Contains(got, "node_modules/") {
		t.Errorf("用户的规则没了：%q", got)
	}
}

// 已经初始化过的目录，计划里那些条目标成「已经在了」而不是消失。
//
// ★ 用户要看到的是「最终会变成什么样」，不是「这次改了几个字节」。
func TestMakePlan_MarksExistingItems(t *testing.T) {
	root := gitRepo(t)
	if err := project.Apply(planFor(t, root)); err != nil {
		t.Fatal(err)
	}

	p := planFor(t, root)
	if len(p.Actions) == 0 {
		t.Fatal("第二次算出来的计划是空的——用户看不到「已经建好了什么」")
	}
	if len(p.Pending()) != 0 {
		t.Errorf("已经初始化过却还有 %d 步要做：%+v", len(p.Pending()), p.Pending())
	}
	for _, a := range p.Actions {
		if !a.AlreadyThere {
			t.Errorf("%s 没标成「已经在了」", a.Path)
		}
	}
}

// 相对路径与不存在的目录一律拒绝。
func TestMakePlan_RejectsBadRoots(t *testing.T) {
	if _, err := project.MakePlan("relative/path"); !errors.Is(err, project.ErrPathNotAbsolute) {
		t.Errorf("相对路径的错误 = %v，想要 ErrPathNotAbsolute", err)
	}
	missing := filepath.Join(t.TempDir(), "根本不存在")
	if _, err := project.MakePlan(missing); !errors.Is(err, project.ErrNotADirectory) {
		t.Errorf("不存在目录的错误 = %v，想要 ErrNotADirectory", err)
	}
	// 指向一个文件而不是目录
	f := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := project.MakePlan(f); !errors.Is(err, project.ErrNotADirectory) {
		t.Errorf("文件路径的错误 = %v", err)
	}
}

// `.git` 是文件（worktree / submodule）时也算仓库。
func TestMakePlan_GitFileCountsAsRepo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: ../real\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := planFor(t, root); !p.IsGitRepo {
		t.Error("worktree 里的 .git 是文件——把它报成非仓库的话，那些项目都拿不到 .gitignore 规则")
	}
}

func snapshotPaths(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		out[path] = true
		return nil
	})
	return out
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 %s: %v", p, err)
	}
	return string(b)
}

func treeOf(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		b.WriteString("  " + rel)
		if d.IsDir() {
			b.WriteString("/")
		}
		b.WriteString("\n")
		return nil
	})
	return b.String()
}
