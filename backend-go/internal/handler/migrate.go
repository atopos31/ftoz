package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ftoz/internal/model"
	"ftoz/internal/service"

	"github.com/gin-gonic/gin"
)

const DefaultSourceDir = "/vol1/1000"

var SourceMap = map[string]model.SourceInfo{
	"personal": {Dir: "/vol1/1000", Label: "personal"},
	"team":     {Dir: "/vol1/@team", Label: "team"},
}

// MigrateHandler 迁移处理器
type MigrateHandler struct {
	zimaClient *service.ZimaOSClient
	scanner    *service.Scanner
}

// NewMigrateHandler 创建迁移处理器
func NewMigrateHandler() *MigrateHandler {
	return &MigrateHandler{
		zimaClient: service.NewZimaOSClient(),
		scanner:    service.NewScanner(),
	}
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

	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// 获取 Flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeJSON(w, 500, "不支持流式响应", nil)
		return
	}
	flusher.Flush()

	// 执行迁移
	h.runMigration(w, flusher, req, sourceInfo)
}

func (h *MigrateHandler) runMigration(w http.ResponseWriter, flusher http.Flusher, req *model.MigrateRequest, sourceInfo *model.SourceInfo) {
	storagePath := "/media"
	if req.Storage != "" {
		storagePath = "/media/" + strings.Trim(req.Storage, "/")
	}

	currentStep := "login"

	// 1. 登录
	h.sendEvent(w, flusher, "progress", model.ProgressEvent{
		Step:    "login",
		Status:  "start",
		Message: "正在登录 ZimaOS...",
	})

	token, err := h.zimaClient.Login(req.BaseURL, req.Username, req.Password)
	if err != nil {
		h.sendEvent(w, flusher, "error", model.ErrorEvent{
			Step:    currentStep,
			Message: err.Error(),
		})
		return
	}

	h.sendEvent(w, flusher, "progress", model.ProgressEvent{
		Step:    "login",
		Status:  "success",
		Message: "登录成功",
	})

	// 2. 扫描目录
	currentStep = "scan"
	h.sendEvent(w, flusher, "progress", model.ProgressEvent{
		Step:    "scan",
		Status:  "start",
		Message: "正在扫描目录...",
	})

	files, dirs, err := h.scanner.Scan(sourceInfo.Dir)
	if err != nil {
		h.sendEvent(w, flusher, "error", model.ErrorEvent{
			Step:    currentStep,
			Message: err.Error(),
		})
		return
	}

	totalFiles := len(files)
	h.sendEvent(w, flusher, "progress", model.ProgressEvent{
		Step:       "scan",
		Status:     "success",
		Message:    fmt.Sprintf("扫描完成：%d 个文件", totalFiles),
		TotalFiles: totalFiles,
	})

	// 3. 创建远程目录
	currentStep = "upload"
	supportsMkdir := true
	createdDirs := make(map[string]bool)

	// 按路径长度排序，确保父目录先创建
	sortedDirs := h.sortDirsByDepth(dirs)

	for _, relDir := range sortedDirs {
		if relDir == "" || createdDirs[relDir] {
			continue
		}
		relPosix := toPosixPath(relDir)
		remoteDir := storagePath + "/" + relPosix

		if supportsMkdir {
			err := h.zimaClient.CreateDir(req.BaseURL, token, remoteDir)
			if err != nil {
				if strings.Contains(err.Error(), "404") {
					supportsMkdir = false
				} else {
					h.sendEvent(w, flusher, "error", model.ErrorEvent{
						Step:    currentStep,
						Message: err.Error(),
					})
					return
				}
			}
		}
		createdDirs[relDir] = true
	}

	// 4. 上传文件
	startMsg := "开始上传文件..."
	if totalFiles == 0 {
		startMsg = "无需上传文件"
	}
	h.sendEvent(w, flusher, "progress", model.ProgressEvent{
		Step:       "upload",
		Status:     "start",
		Message:    startMsg,
		TotalFiles: totalFiles,
	})

	for i, relPath := range files {
		relPosix := toPosixPath(relPath)
		fullPath := filepath.Join(sourceInfo.Dir, relPath)
		dirName := filepath.Dir(relPosix)

		remoteDir := storagePath
		if dirName != "." {
			remoteDir = storagePath + "/" + dirName
		}
		filename := filepath.Base(relPosix)

		h.sendEvent(w, flusher, "progress", model.ProgressEvent{
			Step:             "upload",
			Status:           "start",
			Message:          fmt.Sprintf("正在上传 %d/%d", i+1, totalFiles),
			CurrentFile:      relPosix,
			TransferredFiles: i,
			TotalFiles:       totalFiles,
		})

		err := h.zimaClient.UploadFile(req.BaseURL, token, remoteDir, filename, fullPath)
		if err != nil {
			h.sendEvent(w, flusher, "error", model.ErrorEvent{
				Step:    currentStep,
				Message: err.Error(),
			})
			return
		}

		h.sendEvent(w, flusher, "progress", model.ProgressEvent{
			Step:             "upload",
			Status:           "start",
			Message:          fmt.Sprintf("正在上传 %d/%d", i+1, totalFiles),
			CurrentFile:      relPosix,
			TransferredFiles: i + 1,
			TotalFiles:       totalFiles,
		})
	}

	h.sendEvent(w, flusher, "progress", model.ProgressEvent{
		Step:             "upload",
		Status:           "success",
		Message:          "文件上传完成",
		TransferredFiles: totalFiles,
		TotalFiles:       totalFiles,
	})

	// 5. 完成
	h.sendEvent(w, flusher, "done", model.DoneEvent{
		Message: "迁移完成",
		Result: model.MigrateResult{
			DstPath:    storagePath,
			SourceDir:  sourceInfo.Dir,
			SourceType: sourceInfo.Label,
			TotalFiles: totalFiles,
		},
	})
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

func (h *MigrateHandler) sortDirsByDepth(dirs []string) []string {
	sorted := make([]string, len(dirs))
	copy(sorted, dirs)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) < len(sorted[j])
	})
	return sorted
}

func (h *MigrateHandler) sendEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, data interface{}) {
	payload, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\n", eventType)
	fmt.Fprintf(w, "data: %s\n\n", payload)
	flusher.Flush()
}

func (h *MigrateHandler) writeJSON(w http.ResponseWriter, code int, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(model.Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}

func toPosixPath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
