package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 统一请求处理器
type Handler struct {
	migrateHandler *MigrateHandler
	statusHandler  *StatusHandler
	dirHandler     *DirHandler
	readHandler    *ReadHandler
	saveHandler    *SaveHandler
}

// New 创建处理器
func New() *Handler {
	return &Handler{
		migrateHandler: NewMigrateHandler(),
		statusHandler:  NewStatusHandler(),
		dirHandler:     NewDirHandler(),
		readHandler:    NewReadHandler(),
		saveHandler:    NewSaveHandler(),
	}
}

// Migrate 迁移接口
func (h *Handler) Migrate(c *gin.Context) {
	h.migrateHandler.Handle(c)
}

// Status 状态查询接口
func (h *Handler) Status(c *gin.Context) {
	h.statusHandler.Handle(c)
}

// Dir 目录读取接口
func (h *Handler) Dir(c *gin.Context) {
	h.dirHandler.Handle(c)
}

// Read 文件读取接口
func (h *Handler) Read(c *gin.Context) {
	h.readHandler.Handle(c)
}

// Save 文件保存接口
func (h *Handler) Save(c *gin.Context) {
	h.saveHandler.Handle(c)
}

// Dispatch 根据 api-path 或 _api 参数分发请求
func (h *Handler) Dispatch(c *gin.Context) {
	api := c.GetHeader("api-path")
	if api == "" {
		api = c.Query("_api")
	}

	switch api {
	case "migrate":
		h.Migrate(c)
	case "status":
		h.Status(c)
	case "dir":
		h.Dir(c)
	case "read":
		h.Read(c)
	case "save":
		h.Save(c)
	default:
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "不存在的接口",
			"data": nil,
		})
	}
}

// HandleAPI 用于 CGI 模式的 API 处理 (标准 http.Handler)
func (h *Handler) HandleAPI(w http.ResponseWriter, r *http.Request, api string) {
	switch api {
	case "migrate":
		h.migrateHandler.HandleHTTP(w, r)
	case "status":
		h.statusHandler.HandleHTTP(w, r)
	case "dir":
		h.dirHandler.HandleHTTP(w, r)
	case "read":
		h.readHandler.HandleHTTP(w, r)
	case "save":
		h.saveHandler.HandleHTTP(w, r)
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"code":404,"msg":"不存在的接口","data":null}`))
	}
}

// ServeStaticFile 提供静态文件服务
func (h *Handler) ServeStaticFile(w http.ResponseWriter, filePath string, cache bool) {
	h.readHandler.ServeFile(w, filePath, cache)
}
