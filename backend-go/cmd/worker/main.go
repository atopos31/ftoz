package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ftoz/internal/model"
	"ftoz/internal/service"
)

const (
	StatusDir        = "/tmp"
	DefaultSourceDir = "/vol1/1000"
)

var SourceMap = map[string]model.SourceInfo{
	"personal": {Dir: "/vol1/1000", Label: "personal"},
	"team":     {Dir: "/vol1/@team", Label: "team"},
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: worker <taskId> <paramsJson>")
		os.Exit(1)
	}

	taskId := os.Args[1]
	paramsJson := os.Args[2]

	// 解析参数
	var req model.MigrateRequest
	if err := json.Unmarshal([]byte(paramsJson), &req); err != nil {
		updateStatus(taskId, &model.TaskStatus{
			TaskID:     taskId,
			Status:     "error",
			Error:      "参数解析失败: " + err.Error(),
			UpdateTime: time.Now().Unix(),
		})
		os.Exit(1)
	}

	// 执行迁移
	runMigration(taskId, &req)
}

func runMigration(taskId string, req *model.MigrateRequest) {
	zimaClient := service.NewZimaOSClient()
	scanner := service.NewScanner()

	// 验证参数
	req.BaseURL = strings.TrimRight(strings.TrimSpace(req.BaseURL), "/")
	req.Username = strings.TrimSpace(req.Username)
	req.Storage = strings.Trim(strings.TrimSpace(req.Storage), "/")

	if req.BaseURL == "" || req.Username == "" || req.Password == "" {
		updateStatus(taskId, &model.TaskStatus{
			TaskID:     taskId,
			Status:     "error",
			Error:      "缺少 baseUrl/username/password",
			UpdateTime: time.Now().Unix(),
		})
		return
	}

	// 解析源目录
	sourceInfo := resolveSource(req.Source, req.Space)
	if sourceInfo == nil {
		updateStatus(taskId, &model.TaskStatus{
			TaskID:     taskId,
			Status:     "error",
			Error:      "未知的迁移空间",
			UpdateTime: time.Now().Unix(),
		})
		return
	}

	// 验证源目录存在
	stat, err := os.Stat(sourceInfo.Dir)
	if os.IsNotExist(err) {
		updateStatus(taskId, &model.TaskStatus{
			TaskID:     taskId,
			Status:     "error",
			Error:      "源目录不存在",
			UpdateTime: time.Now().Unix(),
		})
		return
	}
	if !stat.IsDir() {
		updateStatus(taskId, &model.TaskStatus{
			TaskID:     taskId,
			Status:     "error",
			Error:      "源路径不是目录",
			UpdateTime: time.Now().Unix(),
		})
		return
	}

	storagePath := "/media"
	if req.Storage != "" {
		storagePath = "/media/" + strings.Trim(req.Storage, "/")
	}

	// 1. 登录
	updateStatus(taskId, &model.TaskStatus{
		TaskID:     taskId,
		Status:     "running",
		Step:       "login",
		Message:    "正在登录 ZimaOS...",
		UpdateTime: time.Now().Unix(),
	})

	token, err := zimaClient.Login(req.BaseURL, req.Username, req.Password)
	if err != nil {
		updateStatus(taskId, &model.TaskStatus{
			TaskID:     taskId,
			Status:     "error",
			Step:       "login",
			Error:      err.Error(),
			UpdateTime: time.Now().Unix(),
		})
		return
	}

	updateStatus(taskId, &model.TaskStatus{
		TaskID:     taskId,
		Status:     "running",
		Step:       "login",
		Message:    "登录成功",
		UpdateTime: time.Now().Unix(),
	})

	// 2. 扫描目录
	updateStatus(taskId, &model.TaskStatus{
		TaskID:     taskId,
		Status:     "running",
		Step:       "scan",
		Message:    "正在扫描目录...",
		UpdateTime: time.Now().Unix(),
	})

	files, dirs, err := scanner.Scan(sourceInfo.Dir)
	if err != nil {
		updateStatus(taskId, &model.TaskStatus{
			TaskID:     taskId,
			Status:     "error",
			Step:       "scan",
			Error:      err.Error(),
			UpdateTime: time.Now().Unix(),
		})
		return
	}

	totalFiles := len(files)
	updateStatus(taskId, &model.TaskStatus{
		TaskID:     taskId,
		Status:     "running",
		Step:       "scan",
		Message:    fmt.Sprintf("扫描完成：%d 个文件", totalFiles),
		TotalFiles: totalFiles,
		UpdateTime: time.Now().Unix(),
	})

	// 3. 创建远程目录
	supportsMkdir := true
	createdDirs := make(map[string]bool)
	sortedDirs := sortDirsByDepth(dirs)

	for _, relDir := range sortedDirs {
		if relDir == "" || createdDirs[relDir] {
			continue
		}
		relPosix := toPosixPath(relDir)
		remoteDir := storagePath + "/" + relPosix

		if supportsMkdir {
			err := zimaClient.CreateDir(req.BaseURL, token, remoteDir)
			if err != nil {
				if strings.Contains(err.Error(), "404") {
					supportsMkdir = false
				} else {
					updateStatus(taskId, &model.TaskStatus{
						TaskID:     taskId,
						Status:     "error",
						Step:       "upload",
						Error:      err.Error(),
						UpdateTime: time.Now().Unix(),
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
	updateStatus(taskId, &model.TaskStatus{
		TaskID:     taskId,
		Status:     "running",
		Step:       "upload",
		Message:    startMsg,
		TotalFiles: totalFiles,
		UpdateTime: time.Now().Unix(),
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

		updateStatus(taskId, &model.TaskStatus{
			TaskID:           taskId,
			Status:           "running",
			Step:             "upload",
			Message:          fmt.Sprintf("正在上传 %d/%d", i+1, totalFiles),
			CurrentFile:      relPosix,
			TransferredFiles: i,
			TotalFiles:       totalFiles,
			UpdateTime:       time.Now().Unix(),
		})

		err := zimaClient.UploadFile(req.BaseURL, token, remoteDir, filename, fullPath)
		if err != nil {
			updateStatus(taskId, &model.TaskStatus{
				TaskID:           taskId,
				Status:           "error",
				Step:             "upload",
				CurrentFile:      relPosix,
				TransferredFiles: i,
				TotalFiles:       totalFiles,
				Error:            err.Error(),
				UpdateTime:       time.Now().Unix(),
			})
			return
		}

		updateStatus(taskId, &model.TaskStatus{
			TaskID:           taskId,
			Status:           "running",
			Step:             "upload",
			Message:          fmt.Sprintf("正在上传 %d/%d", i+1, totalFiles),
			CurrentFile:      relPosix,
			TransferredFiles: i + 1,
			TotalFiles:       totalFiles,
			UpdateTime:       time.Now().Unix(),
		})
	}

	// 5. 完成
	result := model.MigrateResult{
		DstPath:    storagePath,
		SourceDir:  sourceInfo.Dir,
		SourceType: sourceInfo.Label,
		TotalFiles: totalFiles,
	}
	updateStatus(taskId, &model.TaskStatus{
		TaskID:           taskId,
		Status:           "success",
		Step:             "done",
		Message:          "迁移完成",
		TransferredFiles: totalFiles,
		TotalFiles:       totalFiles,
		Result:           &result,
		UpdateTime:       time.Now().Unix(),
	})
}

func updateStatus(taskId string, status *model.TaskStatus) {
	statusFile := filepath.Join(StatusDir, fmt.Sprintf("ftoz-migrate-%s.json", taskId))
	data, _ := json.Marshal(status)
	os.WriteFile(statusFile, data, 0644)
}

func resolveSource(sourceType, space string) *model.SourceInfo {
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

func sortDirsByDepth(dirs []string) []string {
	sorted := make([]string, len(dirs))
	copy(sorted, dirs)
	sort.Slice(sorted, func(i, j int) bool {
		return len(sorted[i]) < len(sorted[j])
	})
	return sorted
}

func toPosixPath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}
