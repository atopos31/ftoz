package model

// MigrateRequest 迁移请求参数
type MigrateRequest struct {
	BaseURL  string `json:"baseUrl"`
	Username string `json:"username"`
	Password string `json:"password"`
	Storage  string `json:"storage"`
	Source   string `json:"source"`
	Space    string `json:"space"` // 兼容旧参数名
}

// DirRequest 目录读取请求参数
type DirRequest struct {
	Path string `form:"path" json:"path"`
}

// ReadRequest 文件读取请求参数
type ReadRequest struct {
	Path  string `form:"path"`
	Cache string `form:"cache"`
}

// SaveRequest 文件保存请求参数
type SaveRequest struct {
	Path   string `json:"path"`
	Value  string `json:"value"`
	Encode string `json:"encode"`
	Force  int    `json:"force"`
}

// SourceInfo 源目录信息
type SourceInfo struct {
	Dir   string
	Label string
}
