package project_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/project"
	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
	"github.com/HuLuca1998/acp-flows/backend/internal/gitx"
	"github.com/HuLuca1998/acp-flows/backend/tests/testutil"
)

// U2.1.1 · 添加本地项目（验收点 V4）
//
// ★ 这里用**真的 gitx 实现**，不塞假的：本单元的第一条禁令是
// 「不往用户项目目录写任何 Duet 自己的文件」，而假实现什么都不写——
// 那条断言会永远绿，也就永远测不到真正的风险。
//
// 仓储用内存实现是**恰当**的：落库行为在 store 包用真 SQLite 测，
// 这一层要验的是用例编排。

// memRepo 是内存仓储。它只负责「存过的能取回来」，不模拟任何失败。
type memRepo struct {
	items []*model.Project
	// saves 记录写了几次，用来证明重复添加没有落第二条
	saves int
}

func (r *memRepo) SaveProject(_ context.Context, p *model.Project) error {
	r.saves++
	for i, existing := range r.items {
		if existing.ID() == p.ID() {
			r.items[i] = p
			return nil
		}
	}
	r.items = append(r.items, p)
	return nil
}

func (r *memRepo) ListProjects(context.Context) ([]*model.Project, error) {
	return r.items, nil
}

func (r *memRepo) FindProjectByPath(_ context.Context, path string) (*model.Project, error) {
	for _, p := range r.items {
		if p.Path() == path {
			return p, nil
		}
	}
	return nil, model.ErrNotFound
}

func (r *memRepo) DeleteProject(_ context.Context, id string) error {
	for i, p := range r.items {
		if p.ID() == id {
			r.items = append(r.items[:i], r.items[i+1:]...)
			return nil
		}
	}
	return model.ErrNotFound
}

// seqIDs 生成可预测的 ID，让断言能写死。
type seqIDs struct{ n int }

func (g *seqIDs) NextID(prefix string) string {
	g.n++
	return prefix + "-" + string(rune('0'+g.n))
}
func (g *seqIDs) NextULID() string { return g.NextID("ulid") }

// gitAdapter 把 gitx 接到 port 上。生产接线也走这一层。
type gitAdapter struct{}

func (gitAdapter) ProbeGit(ctx context.Context, path string) (port.GitInfo, error) {
	info, err := gitx.Probe(ctx, path)
	if errors.Is(err, gitx.ErrNotADirectory) {
		// 基础设施的错误类型不许穿到 app/api——翻成契约里的哨兵
		return port.GitInfo{}, fmt.Errorf("%w: %s", port.ErrPathNotFound, path)
	}
	return port.GitInfo{IsRepo: info.IsRepo, DefaultBranch: info.DefaultBranch}, err
}

func newService(repo *memRepo) *project.Service {
	return project.New(repo, gitAdapter{}, &seqIDs{})
}

// ★★ 本单元最要紧的一条（R2）：添加项目往用户目录里写**零个字节**。
//
// 顺手初始化 `.acpflows/` 目录结构是很自然的想法，但用户刚把自己的仓库
// 加进来、`git status` 就多出一堆没见过的东西，是最快失去信任的方式。
func TestAdd_WritesNothingIntoUserProject(t *testing.T) {
	repo := testutil.NewGitRepo(t)
	before := testutil.SnapshotDir(t, repo)

	svc := newService(&memRepo{})
	if _, err := svc.Add(context.Background(), repo); err != nil {
		t.Fatalf("添加失败: %v", err)
	}

	testutil.AssertUnchanged(t, repo, before)
}

func TestAdd(t *testing.T) {
	t.Run("git 仓库：记下默认分支，不需要修复提示", func(t *testing.T) {
		dir := testutil.NewGitRepo(t)
		svc := newService(&memRepo{})

		p, err := svc.Add(context.Background(), dir)
		if err != nil {
			t.Fatalf("添加失败: %v", err)
		}
		if !p.IsGitRepo() {
			t.Error("IsGitRepo = false，但这是个真仓库")
		}
		if p.DefaultBranch() != "main" {
			t.Errorf("DefaultBranch = %q, 想要 main", p.DefaultBranch())
		}
		if p.Name() != filepath.Base(dir) {
			t.Errorf("Name = %q, 想要目录名 %q", p.Name(), filepath.Base(dir))
		}
	})

	// R3：非 git 目录不拒绝，但要给出**能直接敲的命令**。
	t.Run("普通目录：能加进来，并给出 git init", func(t *testing.T) {
		dir := t.TempDir()
		svc := newService(&memRepo{})

		p, err := svc.Add(context.Background(), dir)
		if err != nil {
			t.Fatalf("普通目录应当能加进来: %v", err)
		}
		if p.IsGitRepo() {
			t.Error("IsGitRepo = true，但这只是个普通目录")
		}
		// 提示要含具体命令，不是「请检查配置」
		if got := svc.Remedy(p); got != "git init" {
			t.Errorf("Remedy = %q, 想要 git init", got)
		}
	})

	t.Run("路径不存在：报错，不落库", func(t *testing.T) {
		repo := &memRepo{}
		svc := newService(repo)

		_, err := svc.Add(context.Background(), filepath.Join(t.TempDir(), "nope"))
		// ★ 要能判定成「路径不存在」，不能只是「出错了」——
		// 界面上「这个文件夹找不到」和「检测失败了」是两句话，
		// 用户能自己解决的只有前者
		if !errors.Is(err, port.ErrPathNotFound) {
			t.Fatalf("err = %v, 想要 port.ErrPathNotFound", err)
		}
		if len(repo.items) != 0 {
			t.Errorf("失败了却落了 %d 条记录", len(repo.items))
		}
	})

	t.Run("相对路径：报错", func(t *testing.T) {
		svc := newService(&memRepo{})

		if _, err := svc.Add(context.Background(), "work/my-app"); !errors.Is(err, model.ErrProjectPathNotAbsolute) {
			t.Errorf("err = %v, 想要 ErrProjectPathNotAbsolute", err)
		}
	})
}

// 幂等：同一个目录的不同写法只产生一条记录。
//
// 用户从 Finder 拖两次是很常见的，列表里冒出两条一模一样的项目会让人
// 以为自己点错了，而删掉一条又不知道会不会连带删掉另一条的数据。
func TestAdd_IsIdempotent(t *testing.T) {
	dir := testutil.NewGitRepo(t)
	repo := &memRepo{}
	svc := newService(repo)

	first, err := svc.Add(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Add(context.Background(), dir+"/")
	if err != nil {
		t.Fatalf("重复添加应当返回已有记录而不是报错: %v", err)
	}

	if first.ID() != second.ID() {
		t.Errorf("同一个目录拿到两个 ID：%q 与 %q", first.ID(), second.ID())
	}
	if len(repo.items) != 1 {
		t.Errorf("落了 %d 条记录，想要 1 条", len(repo.items))
	}
}

// 移除只取消登记，**绝不删用户的文件**。
func TestRemove_DoesNotDeleteUserFiles(t *testing.T) {
	dir := testutil.NewGitRepo(t)
	repo := &memRepo{}
	svc := newService(repo)

	p, err := svc.Add(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	before := testutil.SnapshotDir(t, dir)

	if err := svc.Remove(context.Background(), p.ID()); err != nil {
		t.Fatalf("移除失败: %v", err)
	}

	if len(repo.items) != 0 {
		t.Errorf("移除后仍有 %d 条记录", len(repo.items))
	}
	// 用户的目录必须原封不动——这是「移除」与「删除」的全部区别
	testutil.AssertUnchanged(t, dir, before)
}
