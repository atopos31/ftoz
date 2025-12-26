package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ftoz/internal/model"
	"ftoz/internal/util"

	"github.com/gin-gonic/gin"
)

// SaveHandler 文件保存处理器
type SaveHandler struct{}

// NewSaveHandler 创建文件保存处理器
func NewSaveHandler() *SaveHandler {
	return &SaveHandler{}
}

// Handle Gin 处理函数
func (h *SaveHandler) Handle(c *gin.Context) {
	var req model.SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "请求参数解析失败",
			"data": nil,
		})
		return
	}

	result := h.handleSave(&req)
	c.JSON(http.StatusOK, result)
}

// HandleHTTP 标准 HTTP 处理函数 (用于 CGI)
func (h *SaveHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	var req model.SaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(model.Response{
			Code: 400,
			Msg:  "请求参数解析失败",
			Data: nil,
		})
		return
	}

	result := h.handleSave(&req)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(result)
}

func (h *SaveHandler) handleSave(req *model.SaveRequest) model.Response {
	path := req.Path
	if path == "" {
		return model.Response{Code: 400, Msg: "缺少文件路径参数", Data: req}
	}

	// 确保路径以 / 开头
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 检查文件是否存在
	stat, err := os.Stat(path)
	if err == nil {
		// 文件存在，检查是否是文件
		if !stat.Mode().IsRegular() {
			return model.Response{Code: 400, Msg: "路径不是文件", Data: req}
		}
	} else if os.IsNotExist(err) {
		// 文件不存在
		if req.Force == 1 {
			// 强制创建，先创建目录
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return model.Response{Code: 400, Msg: "创建目录失败: " + err.Error(), Data: req}
			}
		} else {
			return model.Response{Code: 404, Msg: "文件不存在", Data: req}
		}
	} else {
		// 其他错误
		if os.IsPermission(err) {
			return model.Response{Code: 401, Msg: "权限不足，无法写入文件", Data: req}
		}
		return model.Response{Code: 400, Msg: "文件操作错误: " + err.Error(), Data: req}
	}

	// 编码转换
	encode := req.Encode
	if encode == "" {
		encode = "utf-8"
	}

	data, err := util.EncodeString(req.Value, encode)
	if err != nil {
		return model.Response{Code: 400, Msg: "编码转换失败: " + err.Error(), Data: req}
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		if os.IsPermission(err) {
			return model.Response{Code: 401, Msg: "权限不足，无法写入文件", Data: req}
		}
		return model.Response{Code: 400, Msg: "文件操作错误: " + err.Error(), Data: req}
	}

	return model.Response{Code: 200, Msg: "操作成功", Data: nil}
}
