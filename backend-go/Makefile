.PHONY: all build build-server build-cgi clean tidy

# 输出目录 (与 Node.js 版本保持一致)
OUTPUT_DIR = ../app/app/server

# 默认目标
all: build-server build-cgi

# 下载依赖
tidy:
	go mod tidy

# 构建 HTTP 服务
build-server: tidy
	go build -o bin/server ./cmd/server

# 构建 CGI 程序到目标目录 (与 pkg 构建保持一致)
# 对应: pkg -t node18-linux-x64 ./src/cgi.js --output ../app/app/server/api
build: tidy
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(OUTPUT_DIR)/api ./cmd/cgi

# 构建 CGI 程序 (静态链接，适用于飞牛系统)
build-cgi: tidy
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/index.cgi ./cmd/cgi

# 构建 CGI 程序 (本地测试)
build-cgi-local: tidy
	CGO_ENABLED=0 go build -o bin/index.cgi ./cmd/cgi

# 清理构建产物
clean:
	rm -rf bin/
	rm -f $(OUTPUT_DIR)/api

# 运行开发服务器
run: build-server
	./bin/server

# 帮助信息
help:
	@echo "可用目标:"
	@echo "  build         - 构建 CGI 到 $(OUTPUT_DIR)/api (与 Node.js pkg 一致)"
	@echo "  all           - 构建所有目标 (server + cgi)"
	@echo "  build-server  - 构建 HTTP 服务"
	@echo "  build-cgi     - 构建 CGI 程序到 bin/ (Linux amd64)"
	@echo "  build-cgi-local - 构建 CGI 程序 (本地平台)"
	@echo "  clean         - 清理构建产物"
	@echo "  run           - 运行开发服务器"
	@echo "  tidy          - 下载依赖"
