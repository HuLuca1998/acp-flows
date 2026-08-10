package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// prepareWorkRequest 是 POST /v1/works/prepare 的请求体。
type prepareWorkRequest struct {
	Project string `json:"project"`
}

// workPrepBody 对应 openapi 的 WorkPreparation。
type workPrepBody struct {
	CurrentBranch string   `json:"current_branch"`
	Branches      []string `json:"branches"`
	HeadCommit    string   `json:"head_commit"`
	// ★ 已跟踪与未跟踪**分开**：合成一条的话，「新建了几个还没 add 的文件」
	// 和「改了正在跟踪的代码」会长得一模一样，而对用户是两件不同的事。
	TrackedDirty int `json:"tracked_dirty"`
	Untracked    int `json:"untracked"`
}

// handlePrepareWork 处理 POST /v1/works/prepare。
//
// ★★ **只看不动**：用户还没决定开不开工。
func handlePrepareWork(svc workService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"work_service_unavailable", "Work service is not configured")
			return
		}

		var req prepareWorkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_request_body", "malformed JSON")
			return
		}
		if strings.TrimSpace(req.Project) == "" {
			writeProblem(w, http.StatusBadRequest, "work_project_required", "project is required")
			return
		}

		st, err := svc.Prepare(r.Context(), req.Project)
		if err != nil {
			// ★ rebase / merge 中途、空仓库、非仓库——都在这里如实报出去。
			// 含糊成一句「准备失败」的话，用户不知道自己该做什么。
			writeWorkProblem(w, "prepare work", err)
			return
		}

		branches := st.Branches
		if branches == nil {
			// 空集合序列化成 `[]` 不是 null——前端会崩在 `.map` 上
			branches = []string{}
		}
		writeJSON(w, http.StatusOK, workPrepBody{
			CurrentBranch: st.CurrentBranch,
			Branches:      branches,
			HeadCommit:    st.HeadCommit,
			TrackedDirty:  st.TrackedDirty,
			Untracked:     st.Untracked,
		})
	}
}
