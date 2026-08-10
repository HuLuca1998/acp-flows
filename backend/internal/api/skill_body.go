package api

import "net/http"

// skillBodyReader 读一个全局 Skill 的 SKILL.md 正文。
//
// ★ 正文**从磁盘现读**，不从任何缓存（Skill 是用户的产物，
// 他随时可能用编辑器改它）。frontmatter 原样给——解析失败的文件，
// 用户要看得到自己写了什么才改得动。
type skillBodyReader interface {
	ReadGlobalBody(dir string) (frontmatter, text string, err error)
}

// skillBodyResponse 是一个 Skill 的正文。
type skillBodyResponse struct {
	Dir         string `json:"dir"`
	Frontmatter string `json:"frontmatter"`
	Text        string `json:"text"`
}

// handleGetSkillBody 处理 GET /v1/skills/{dir}/body。
//
// ★ 单独一个端点，不塞进列表（与记忆正文同理）：
// 列表逐条读文件是 N 次磁盘 IO。
func handleGetSkillBody(r0 skillBodyReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r0 == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"skill_body_unavailable", "Skill body storage is not configured")
			return
		}
		dir := r.PathValue("dir")
		frontmatter, text, err := r0.ReadGlobalBody(dir)
		if err != nil {
			// ★★ 文件不在要**说清楚**（带路径），不返回空正文：
			// 空正文看起来像「这个 Skill 没写内容」，而真相是「文件丢了」。
			writeProblem(w, http.StatusNotFound, "skill_body_missing", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, skillBodyResponse{
			Dir: dir, Frontmatter: frontmatter, Text: text,
		})
	}
}
