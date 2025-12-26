package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ftoz/internal/model"

	"github.com/gin-gonic/gin"
)

// DirHandler 目录读取处理器
type DirHandler struct{}

// NewDirHandler 创建目录处理器
func NewDirHandler() *DirHandler {
	return &DirHandler{}
}

// Handle Gin 处理函数
func (h *DirHandler) Handle(c *gin.Context) {
	var req model.DirRequest

	// 支持 GET query 和 POST body
	if c.Request.Method == "POST" {
		c.ShouldBindJSON(&req)
	}
	if req.Path == "" {
		req.Path = c.Query("path")
	}

	result := h.handleDir(req.Path)
	c.JSON(http.StatusOK, result)
}

// HandleHTTP 标准 HTTP 处理函数 (用于 CGI)
func (h *DirHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	var req model.DirRequest

	if r.Method == "POST" {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Path == "" {
		req.Path = r.URL.Query().Get("path")
	}

	result := h.handleDir(req.Path)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(result)
}

func (h *DirHandler) handleDir(path string) model.Response {
	if path == "" {
		return model.Response{Code: 400, Msg: "缺少文件路径参数", Data: nil}
	}

	// 确保路径以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 检查目录是否存在
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return model.Response{Code: 404, Msg: "目录不存在", Data: nil}
	}
	if err != nil {
		if os.IsPermission(err) {
			return model.Response{Code: 403, Msg: "权限不足，无法读取目录", Data: nil}
		}
		return model.Response{Code: 500, Msg: "读取目录失败: " + err.Error(), Data: nil}
	}

	if !stat.IsDir() {
		return model.Response{Code: 400, Msg: "路径不是目录", Data: nil}
	}

	// 读取目录内容
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsPermission(err) {
			return model.Response{Code: 403, Msg: "权限不足，无法读取目录", Data: nil}
		}
		return model.Response{Code: 500, Msg: "读取目录失败: " + err.Error(), Data: nil}
	}

	result := model.DirData{
		Files: []string{},
		Dirs:  []string{},
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(path, name)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		if info.IsDir() {
			result.Dirs = append(result.Dirs, name)
		} else {
			result.Files = append(result.Files, name)
		}
	}

	// 按字母排序（不区分大小写）
	sort.Slice(result.Files, func(i, j int) bool {
		return strings.ToLower(result.Files[i]) < strings.ToLower(result.Files[j])
	})
	sort.Slice(result.Dirs, func(i, j int) bool {
		return strings.ToLower(result.Dirs[i]) < strings.ToLower(result.Dirs[j])
	})

	return model.Response{Code: 200, Msg: "操作成功", Data: result}
}
