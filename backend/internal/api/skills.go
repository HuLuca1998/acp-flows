package api

import (
	"net/http"

	"github.com/HuLuca1998/acp-flows/backend/internal/app/port"
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
	HitCount         int    `json:"hit_count,omitempty"`
}

type skillsBody struct {
	Skills []skillBody `json:"skills"`
}

// handleListSkills 处理 GET /v1/skills。
func handleListSkills(s port.SkillScanner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"skills_unavailable", "Skill library is not configured")
			return
		}

		// ★ M2 只有全局库。项目级的在创建项目时初始化（M3），
		// 那时才有项目——现在支持 scope=project 只会返回一个永远的空列表，
		// 而用户会以为自己的项目 skill 没被认出来。
		if scope := r.URL.Query().Get("scope"); scope == "project" {
			writeProblem(w, http.StatusNotImplemented,
				"project_skills_not_ready", "Project-level skills arrive with project creation")
			return
		}

		entries, err := s.ScanGlobal()
		if err != nil {
			// ★ 扫不动要说出来，不装作「一个都没有」——
			// 装作没有的话用户以为自己的 skill 丢了，而实际是目录读不了。
			writeProblem(w, http.StatusInternalServerError,
				"skill_scan_failed", err.Error())
			return
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
			})
		}
		writeJSON(w, http.StatusOK, body)
	}
}
