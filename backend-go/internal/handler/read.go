package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ftoz/internal/model"
	"ftoz/internal/util"

	"github.com/gin-gonic/gin"
)

// ReadHandler 文件读取处理器
type ReadHandler struct{}

// NewReadHandler 创建文件读取处理器
func NewReadHandler() *ReadHandler {
	return &ReadHandler{}
}

// Handle Gin 处理函数
func (h *ReadHandler) Handle(c *gin.Context) {
	path := c.Query("path")
	cache := c.Query("cache")

	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "缺少文件路径参数",
			"data": nil,
		})
		return
	}

	h.serveFileWithResponse(c.Writer, path, cache != "")
}

// HandleHTTP 标准 HTTP 处理函数 (用于 CGI)
func (h *ReadHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	cache := r.URL.Query().Get("cache")

	if path == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(model.Response{
			Code: 400,
			Msg:  "缺少文件路径参数",
			Data: nil,
		})
		return
	}

	h.serveFileWithResponse(w, path, cache != "")
}

// ServeFile 提供静态文件服务 (用于 CGI 静态资源)
func (h *ReadHandler) ServeFile(w http.ResponseWriter, path string, cache bool) {
	h.serveFileWithResponse(w, path, cache)
}

func (h *ReadHandler) serveFileWithResponse(w http.ResponseWriter, path string, cache bool) {
	// 确保路径以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 检查文件是否存在
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		h.writeError(w, 404, "文件不存在")
		return
	}
	if err != nil {
		if os.IsPermission(err) {
			h.writeError(w, 403, "权限不足，无法读取文件")
			return
		}
		h.writeError(w, 500, "读取文件失败: "+err.Error())
		return
	}

	if stat.IsDir() {
		h.writeError(w, 400, "路径是目录而不是文件")
		return
	}

	// 打开文件
	file, err := os.Open(path)
	if err != nil {
		if os.IsPermission(err) {
			h.writeError(w, 403, "权限不足，无法读取文件")
			return
		}
		h.writeError(w, 500, "读取文件失败: "+err.Error())
		return
	}
	defer file.Close()

	// 设置响应头
	filename := filepath.Base(path)
	contentType := util.GetMimeType(filename)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))

	// 设置缓存头
	if cache {
		maxAge := 365 * 24 * 60 * 60 // 1 year
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", maxAge))
		w.Header().Set("Expires", time.Now().Add(time.Duration(maxAge)*time.Second).UTC().Format(http.TimeFormat))
		w.Header().Set("Last-Modified", stat.ModTime().UTC().Format(http.TimeFormat))
		w.Header().Set("ETag", fmt.Sprintf(`"%d-%d"`, stat.Size(), stat.ModTime().UnixMilli()))
	}

	// 流式传输文件内容
	io.Copy(w, file)
}

func (h *ReadHandler) writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(model.Response{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}
