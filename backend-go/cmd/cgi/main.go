package main

import (
	"net/http"
	"net/http/cgi"
	"os"
	"strings"

	"ftoz/internal/handler"
)

const (
	cgiPrefix = "/cgi/ThirdParty/ftoz/index.cgi"
	distDir   = "/var/apps/ftoz/target/server/dist"
)

func main() {
	if err := cgi.Serve(http.HandlerFunc(cgiHandler)); err != nil {
		// CGI 错误输出
		os.Stdout.WriteString("Content-Type: application/json; charset=utf-8\n\n")
		os.Stdout.WriteString(`{"code":500,"msg":"CGI 服务错误","data":null}`)
	}
}

func cgiHandler(w http.ResponseWriter, r *http.Request) {
	h := handler.New()

	// 获取 PATH_INFO
	pathInfo := os.Getenv("PATH_INFO")
	assets := strings.TrimPrefix(pathInfo, cgiPrefix)

	// 处理静态资源请求
	if assets != "" && assets != pathInfo {
		filePath := distDir + assets
		if assets == "/" {
			filePath = distDir + "/index.html"
		}
		cache := assets != "/"
		h.ServeStaticFile(w, filePath, cache)
		return
	}

	// 获取 API 名称
	api := r.Header.Get("api-path")
	if api == "" {
		api = r.URL.Query().Get("_api")
	}

	// 处理 API 请求
	h.HandleAPI(w, r, api)
}
