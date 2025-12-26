package handler

import (
	"encoding/json"
	"net/http"

	"ftoz/internal/model"

	"github.com/gin-gonic/gin"
)

// StatusHandler 状态查询处理器
type StatusHandler struct{}

// NewStatusHandler 创建状态处理器
func NewStatusHandler() *StatusHandler {
	return &StatusHandler{}
}

// Handle Gin 处理函数
func (h *StatusHandler) Handle(c *gin.Context) {
	taskId := c.Query("taskId")
	h.handleStatus(c.Writer, taskId)
}

// HandleHTTP 标准 HTTP 处理函数 (用于 CGI)
func (h *StatusHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	taskId := r.URL.Query().Get("taskId")
	h.handleStatus(w, taskId)
}

func (h *StatusHandler) handleStatus(w http.ResponseWriter, taskId string) {
	if taskId == "" {
		h.writeJSON(w, 400, "缺少 taskId 参数", nil)
		return
	}

	// 读取状态文件
	status, err := readStatusFile(taskId)
	if err != nil {
		h.writeJSON(w, 404, "任务不存在", nil)
		return
	}

	h.writeJSON(w, 200, "操作成功", status)
}

func (h *StatusHandler) writeJSON(w http.ResponseWriter, code int, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(model.Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}
