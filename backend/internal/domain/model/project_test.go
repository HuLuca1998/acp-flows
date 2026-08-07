package model_test

import (
	"errors"
	"testing"

	"github.com/HuLuca1998/acp-flows/backend/internal/domain/model"
)

// U2.1.1 · 项目（验收点 V4）
//
// ★ 这一层最要紧的不是「能不能加进来」，而是**加进来之后什么都没发生**：
// Duet 不往用户的仓库里写一个字节。那条不变量在 app 层用真目录验，
// 这里守的是它的前提——领域模型自己不持有、不构造任何「要写进去的路径」。

func TestNewProject(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		path    string
		wantErr error
		// wantName 为空表示不检查
		wantName string
	}{
		{
			name:     "正常路径：名字取目录名",
			id:       "proj-01",
			path:     "/Users/me/work/my-app",
			wantName: "my-app",
		},
		{
			// ★ 必须是绝对路径。相对路径会在 duetd 的工作目录下解析，
			// 而那是个用户完全不知道的位置——今天能用，换个启动方式就指到别处，
			// 最坏情况是 Duet 在一个用户没预期的目录里开工作区。
			name:    "相对路径被拒",
			id:      "proj-01",
			path:    "work/my-app",
			wantErr: model.ErrProjectPathNotAbsolute,
		},
		{
			name:    "空路径被拒",
			id:      "proj-01",
			path:    "",
			wantErr: model.ErrProjectPathNotAbsolute,
		},
		{
			name:    "空 ID 被拒",
			id:      "",
			path:    "/Users/me/work/my-app",
			wantErr: model.ErrProjectIDRequired,
		},
		{
			// 末尾斜杠是用户从 Finder 拖进来时的常见形态，
			// 不规整的话同一个目录会被当成两个不同的项目加两次。
			name:     "末尾斜杠被规整掉，名字仍取目录名",
			id:       "proj-02",
			path:     "/Users/me/work/my-app/",
			wantName: "my-app",
		},
		{
			// 根目录没有「目录名」可取。允许添加但名字得有个说得过去的值，
			// 不能是空字符串——界面上会显示成一行空白。
			name:     "根目录：名字不能是空字符串",
			id:       "proj-03",
			path:     "/",
			wantName: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := model.NewProject(tt.id, tt.path)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, 想要 %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("意外失败: %v", err)
			}
			if tt.wantName != "" && p.Name() != tt.wantName {
				t.Errorf("Name = %q, 想要 %q", p.Name(), tt.wantName)
			}
			if p.ID() != tt.id {
				t.Errorf("ID = %q, 想要 %q", p.ID(), tt.id)
			}
		})
	}
}

// 同一个目录的不同写法应当规整成同一个 Path——否则用户会把一个项目加两次，
// 然后在列表里看到两条一模一样的记录。
func TestProject_PathIsNormalized(t *testing.T) {
	variants := []string{
		"/Users/me/work/my-app",
		"/Users/me/work/my-app/",
		"/Users/me/work/./my-app",
		"/Users/me/work/other/../my-app",
	}

	var first string
	for i, v := range variants {
		p, err := model.NewProject("proj-01", v)
		if err != nil {
			t.Fatalf("%q: %v", v, err)
		}
		if i == 0 {
			first = p.Path()
			continue
		}
		if p.Path() != first {
			t.Errorf("%q 规整成 %q，而 %q 规整成 %q——同一个目录会被加两次",
				v, p.Path(), variants[0], first)
		}
	}
}

// ★ 用户可以改显示名，但**不影响 path**。
// 反过来（改名跟着改路径）会让 Duet 去操作一个不存在的目录。
func TestProject_RenameDoesNotTouchPath(t *testing.T) {
	p, err := model.NewProject("proj-01", "/Users/me/work/my-app")
	if err != nil {
		t.Fatal(err)
	}
	before := p.Path()

	if err := p.Rename("我的应用"); err != nil {
		t.Fatalf("改名失败: %v", err)
	}

	if p.Name() != "我的应用" {
		t.Errorf("Name = %q", p.Name())
	}
	if p.Path() != before {
		t.Errorf("Path 被改名带跑了：%q → %q", before, p.Path())
	}
}

// 空名字会在界面上显示成一行空白，等于这条记录消失了。
func TestProject_RenameRejectsBlank(t *testing.T) {
	p, err := model.NewProject("proj-01", "/Users/me/work/my-app")
	if err != nil {
		t.Fatal(err)
	}

	for _, blank := range []string{"", "   ", "\t\n"} {
		if err := p.Rename(blank); !errors.Is(err, model.ErrProjectNameRequired) {
			t.Errorf("Rename(%q) 的 err = %v, 想要 ErrProjectNameRequired", blank, err)
		}
		if p.Name() != "my-app" {
			t.Fatalf("改名失败了却把名字改掉了：%q", p.Name())
		}
	}
}
