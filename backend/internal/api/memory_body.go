package api

import "net/http"

// memoryBodyReader 读一条记忆的正文。
//
// ★★ 正文从 **md 文件**读，不从数据库（INV-MEM-8）。
// 但「不进数据库」不等于「API 不能给」——用户要读到正文才决定得了
// 收不收这条记忆，不给他就是在盲选。
type memoryBodyReader interface {
	ReadBody(id string) (title, text string, err error)
}

// memoryBodyResponse 是一条记忆的正文。
type memoryBodyResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

// handleGetMemoryBody 处理 GET /v1/memories/{id}/body。
//
// ★ 单独一个端点，**不塞进列表**：列表里逐条读文件是 N 次磁盘 IO，
// 而记忆页一屏能列几十条——用户会看到一个转很久的圈。
func handleGetMemoryBody(r0 memoryBodyReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r0 == nil {
			writeProblem(w, http.StatusServiceUnavailable,
				"memory_body_unavailable", "Memory body storage is not configured")
			return
		}
		id := r.PathValue("id")
		title, text, err := r0.ReadBody(id)
		if err != nil {
			// ★★ 文件不在要**说清楚**，不返回空正文：空正文看起来像
			// 「这条记忆没内容」，而真相是「文件丢了」——
			// 前者用户会直接把它收下，后者他该去看看磁盘上出了什么事。
			writeProblem(w, http.StatusNotFound, "memory_body_missing", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, memoryBodyResponse{ID: id, Title: title, Text: text})
	}
}
