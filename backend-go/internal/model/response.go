package model

// Response 通用响应结构
type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

// DirData 目录读取响应数据
type DirData struct {
	Files []string `json:"files"`
	Dirs  []string `json:"dirs"`
}

// ProgressEvent SSE 进度事件
type ProgressEvent struct {
	Step             string `json:"step"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	CurrentFile      string `json:"currentFile,omitempty"`
	TransferredFiles int    `json:"transferredFiles,omitempty"`
	TotalFiles       int    `json:"totalFiles,omitempty"`
}

// DoneEvent SSE 完成事件
type DoneEvent struct {
	Message string        `json:"message"`
	Result  MigrateResult `json:"result"`
}

// MigrateResult 迁移结果
type MigrateResult struct {
	DstPath    string `json:"dstPath"`
	SourceDir  string `json:"sourceDir"`
	SourceType string `json:"sourceType"`
	TotalFiles int    `json:"totalFiles"`
}

// ErrorEvent SSE 错误事件
type ErrorEvent struct {
	Step    string `json:"step"`
	Message string `json:"message"`
}
