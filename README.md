# FTOZ

应用包名：ftoz

显示名称：ftoz

将 FNOS 本地目录逐文件迁移到 ZimaOS，保留目录结构，支持个人/团队空间与后台进度查询。

![示例](image-2.png)

## 功能

- 登录 ZimaOS、扫描目录、逐文件上传
- 迁移进度轮询（login / scan / upload / done）
- 支持 personal / team 空间，可用 `SOURCE_DIR` 自定义源目录

## 目录结构

- `frontend/`：前端（Vue 3 + Vite）
- `backend-go/`：Go 后端（CGI + worker，当前实现）
- `backend/`：Node 后端（旧版 SSE，保留作参考）
- `app/`：打包资源

## 本地运行（开发）

> 需要 Node.js 18+；Go 1.24+ 用于 Go 后端。

```bash
npm run install
npm run dev:frontend

# 另起终端运行 Go 后端
cd backend-go
go run ./cmd/server
```

> Go 后端会启动 `/var/apps/ftoz/target/server/worker`。
> 本地调试可先构建 worker 并建立软链接：

```bash
make -C backend-go build-worker-local
sudo mkdir -p /var/apps/ftoz/target/server
sudo ln -sf "$PWD/backend-go/bin/worker" /var/apps/ftoz/target/server/worker
```

> 若使用 `backend/`（SSE 版本），需自行适配前端的轮询逻辑。

## 本地构建 / 打包

> 需提前安装 [fnpack](https://developer.fnnas.com/docs/cli/fnpack)。

```bash
npm run install
npm run build:frontend
make -C backend-go build
fnpack build app
```

产物会生成 `ftoz.fpk`，后端二进制位于 `app/app/server/api` 和 `app/app/server/worker`。

## 迁移接口

开发环境：

```
POST http://127.0.0.1:17746/migrate
```

部署后（CGI）：

```
POST /cgi/ThirdParty/ftoz/index.cgi?_api=migrate
```

请求体（JSON）：

```json
{
  "baseUrl": "http://<zimaos_host>:<port>",
  "username": "你的用户名",
  "password": "你的密码",
  "storage": "ZimaOS-HD",
  "source": "personal"
}
```

说明：
- `storage` 上传路径为 `/media/<storage>`；为空时为 `/media`
- `source` 取值：`personal`（个人空间 `/vol1/1000`）或 `team`（团队空间 `/vol1/@team`）
- 默认迁移目录为 `/vol1/1000`，可通过设置 `SOURCE_DIR` 环境变量修改
- 支持兼容参数 `space`（与 `source` 同义）

响应示例（JSON）：

```json
{
  "code": 200,
  "msg": "迁移任务已启动",
  "data": { "taskId": "xxxx" }
}
```

## 迁移状态查询

开发环境：

```
GET http://127.0.0.1:17746/status?taskId=<taskId>
```

部署后（CGI）：

```
GET /cgi/ThirdParty/ftoz/index.cgi?_api=status&taskId=<taskId>
```

返回示例（JSON）：

```json
{
  "code": 200,
  "msg": "操作成功",
  "data": {
    "status": "running",
    "step": "upload",
    "message": "正在上传 3/10",
    "currentFile": "Photos/1.jpg",
    "transferredFiles": 3,
    "totalFiles": 10
  }
}
```

## 用户使用

1. 在 FNOS 上安装应用（手动安装 `ftoz.fpk`）。
   ![安装](image-1.png)
2. 填写 `Base URL`、用户名、密码、存储名称。
3. 选择迁移空间（个人空间或团队空间）。
4. 点击“开始迁移”，等待进度完成。
   ![进度](image-2.png)
5. 迁移完成后，文件将按原目录结构同步到 `/media/<storage>`。

说明：
- 应用会将 `/vol1/1000` 逐文件迁移，如需变更迁移目录可设置 `SOURCE_DIR` 环境变量。

## 接口调用（可选）

启动迁移：

```bash
curl -X POST 'http://127.0.0.1:17746/migrate' \
  -H 'Content-Type: application/json' \
  -d '{
    "baseUrl": "http://<zimaos_host>:<port>",
    "username": "你的用户名",
    "password": "你的密码",
    "storage": "ZimaOS-HD",
    "source": "personal"
  }'
```

查询状态：

```bash
curl 'http://127.0.0.1:17746/status?taskId=<taskId>'
```

## CGI 模式说明

- 服务端：Go 后端编译为 `api` 与 `worker`，在飞牛以 CGI + 后台任务方式运行。
- 客户端：Vite 打包后的静态资源通过 `index.cgi` 提供访问。
