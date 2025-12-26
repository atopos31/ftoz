package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ftoz/internal/model"

	"github.com/gin-gonic/gin"
)

const (
	DefaultSourceDir = "/vol1/1000"
	StatusDir        = "/tmp"
	WorkerPath       = "/var/apps/ftoz/target/server/worker"
)

var SourceMap = map[string]model.SourceInfo{
	"personal": {Dir: "/vol1/1000", Label: "personal"},
	"team":     {Dir: "/vol1/@team", Label: "team"},
}

// MigrateHandler 迁移处理器
type MigrateHandler struct{}

// NewMigrateHandler 创建迁移处理器
func NewMigrateHandler() *MigrateHandler {
	return &MigrateHandler{}
}

// Handle Gin 处理函数
func (h *MigrateHandler) Handle(c *gin.Context) {
	var req model.MigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "请求参数解析失败",
			"data": nil,
		})
		return
	}

	h.handleMigrate(c.Writer, &req)
}

// HandleHTTP 标准 HTTP 处理函数 (用于 CGI)
func (h *MigrateHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	var req model.MigrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"code":400,"msg":"请求参数解析失败","data":null}`))
		return
	}

	h.handleMigrate(w, &req)
}

func (h *MigrateHandler) handleMigrate(w http.ResponseWriter, req *model.MigrateRequest) {
	// 参数验证
	if err := h.validateRequest(req); err != nil {
		h.writeJSON(w, 400, err.Error(), req)
		return
	}

	// 解析源目录
	sourceInfo := h.resolveSource(req.Source, req.Space)
	if sourceInfo == nil {
		h.writeJSON(w, 400, "未知的迁移空间", req)
		return
	}

	// 验证源目录存在
	stat, err := os.Stat(sourceInfo.Dir)
	if os.IsNotExist(err) {
		h.writeJSON(w, 400, "源目录不存在", nil)
		return
	}
	if !stat.IsDir() {
		h.writeJSON(w, 400, "源路径不是目录", nil)
		return
	}

	// 生成任务ID
	taskId := generateTaskId()

	// 创建初始状态文件
	status := model.TaskStatus{
		TaskID:     taskId,
		Status:     "pending",
		Message:    "任务已创建，等待执行",
		StartTime:  time.Now().Unix(),
		UpdateTime: time.Now().Unix(),
	}
	if err := writeStatusFile(taskId, &status); err != nil {
		h.writeJSON(w, 500, "创建状态文件失败: "+err.Error(), nil)
		return
	}

	// 启动后台进程
	paramsJson, _ := json.Marshal(req)
	cmd := exec.Command(WorkerPath, taskId, string(paramsJson))

	// 设置进程独立运行，不受父进程影响
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		// 更新状态为错误
		status.Status = "error"
		status.Error = "启动后台进程失败: " + err.Error()
		status.UpdateTime = time.Now().Unix()
		writeStatusFile(taskId, &status)

		h.writeJSON(w, 500, "启动迁移任务失败: "+err.Error(), nil)
		return
	}

	// 立即返回任务ID
	h.writeJSON(w, 200, "迁移任务已启动", gin.H{"taskId": taskId})
}

func (h *MigrateHandler) validateRequest(req *model.MigrateRequest) error {
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	req.Username = strings.TrimSpace(req.Username)
	req.Storage = strings.Trim(strings.TrimSpace(req.Storage), "/")

	if req.BaseURL == "" || req.Username == "" || req.Password == "" {
		return fmt.Errorf("缺少 baseUrl/username/password")
	}
	return nil
}

func (h *MigrateHandler) resolveSource(sourceType, space string) *model.SourceInfo {
	// 兼容 source 和 space 参数
	st := strings.TrimSpace(sourceType)
	if st == "" {
		st = strings.TrimSpace(space)
	}

	if info, ok := SourceMap[st]; ok {
		return &info
	}

	if st != "" {
		return nil
	}

	// 默认源目录
	envDir := os.Getenv("SOURCE_DIR")
	if envDir == "" {
		envDir = DefaultSourceDir
	}

	for _, info := range SourceMap {
		if info.Dir == envDir {
			return &info
		}
	}

	return &model.SourceInfo{Dir: envDir, Label: "custom"}
}

func (h *MigrateHandler) writeJSON(w http.ResponseWriter, code int, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(model.Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}

// generateTaskId 生成任务ID
func generateTaskId() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// writeStatusFile 写入状态文件
func writeStatusFile(taskId string, status *model.TaskStatus) error {
	statusFile := filepath.Join(StatusDir, fmt.Sprintf("ftoz-migrate-%s.json", taskId))
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return os.WriteFile(statusFile, data, 0644)
}

// readStatusFile 读取状态文件
func readStatusFile(taskId string) (*model.TaskStatus, error) {
	statusFile := filepath.Join(StatusDir, fmt.Sprintf("ftoz-migrate-%s.json", taskId))
	data, err := os.ReadFile(statusFile)
	if err != nil {
		return nil, err
	}
	var status model.TaskStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}
	return &status, nil
}
