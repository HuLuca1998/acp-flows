package api

import (
	"context"
	"net/http"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
	"github.com/HuLuca1998/acp-flows/backend/internal/app/work"
)

// skillBody 对应 openapi 的 Skill。
type skillBody struct {
	Name             string `json:"name"`
	Dir              string `json:"dir"`
	Version          string `json:"version,omitempty"`
	Description      string `json:"description,omitempty"`
	Compatibility    string `json:"compatibility,omitempty"`
	Scope            string `json:"scope"`
	Source           string `json:"source"`
	Status           string `json:"status"`
	ValidationOK     bool   `json:"validation_ok"`
	ValidationReason string `json:"validation_reason,omitempty"`
	// HitCount 是这个 Skill 被注入过几次。
	//
	// ★★ **没有 omitempty**：0 会被吃掉，而界面上「从没用过」正是要显示
	// 那个 0——空白会让用户以为这个数字坏了。
	HitCount int `json:"hit_count"`
}

type skillsBody struct {
	Skills []skillBody `json:"skills"`
}

// SkillHitsReader 读 Skill 的命中计数。
//
// ★ 可以为 nil，那时一律显示 0——**不是**把这一列藏起来：
// 藏起来的话用户以为这个功能没做。
type SkillHitsReader interface {
	SkillHits(ctx context.Context, refs []string) (map[string]int, error)
}

// handleListSkills 处理 GET /v1/skills。

func handleListSkills(s port.SkillScanner, hits SkillHitsReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"skills_unavailable", "Skill library is not configured")
			return
		}

		// 项目级：给项目路径就扫那个项目（约定目录见 fsstore/skill.Discover）。
		// ★ `scope=project` 不给路径 → 明确报错——回一个永远的空列表的话，
		// 用户以为自己的项目 skill 没被认出来。
		var entries []port.SkillEntry
		var err error
		if scope := r.URL.Query().Get("scope"); scope == "project" {
			projectPath := r.URL.Query().Get("project")
			if projectPath == "" {
				writeProblem(w, http.StatusBadRequest,
					"project_path_required", "scope=project needs a project path")
				return
			}
			entries, err = s.DiscoverInProject(projectPath)
		} else {
			entries, err = s.ScanGlobal()
		}
		if err != nil {
			// ★ 扫不动要说出来，不装作「一个都没有」——
			// 装作没有的话用户以为自己的 skill 丢了，而实际是目录读不了。
			writeProblem(w, http.StatusInternalServerError,
				"skill_scan_failed", err.Error())
			return
		}

		// ★ 计数读不到时**一律 0**，不拦住整个列表：
		// 用户要的是看到自己有哪些 Skill，计数是锦上添花。
		counts := map[string]int{}
		if hits != nil {
			refs := make([]string, 0, len(entries))
			for _, e := range entries {
				refs = append(refs, work.SkillRef(e))
			}
			if got, err := hits.SkillHits(r.Context(), refs); err == nil {
				counts = got
			}
		}

		body := skillsBody{Skills: make([]skillBody, 0, len(entries))}
		for _, e := range entries {
			body.Skills = append(body.Skills, skillBody{
				Name:             e.Name,
				Dir:              e.Dir,
				Version:          e.Version,
				Description:      e.Description,
				Compatibility:    e.Compatibility,
				Scope:            e.Scope,
				Source:           e.Source,
				Status:           e.Status,
				ValidationOK:     e.ValidationOK,
				ValidationReason: e.ValidationReason,
				HitCount:         counts[work.SkillRef(e)],
			})
		}
		writeJSON(w, http.StatusOK, body)
	}
}
