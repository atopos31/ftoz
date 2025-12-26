package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// ZimaOSClient ZimaOS API 客户端
type ZimaOSClient struct {
	client *http.Client
}

// NewZimaOSClient 创建 ZimaOS 客户端
func NewZimaOSClient() *ZimaOSClient {
	return &ZimaOSClient{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Login 登录 ZimaOS 获取 token
func (c *ZimaOSClient) Login(baseURL, username, password string) (string, error) {
	payload := map[string]string{
		"username": username,
		"password": password,
	}
	body, _ := json.Marshal(payload)

	resp, err := c.client.Post(
		baseURL+"/v1/users/login",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", fmt.Errorf("登录请求失败: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析登录响应失败: %w", err)
	}

	if err := c.assertResponse(resp.StatusCode, result, "登录"); err != nil {
		return "", err
	}

	token := c.extractToken(result)
	if token == "" {
		return "", fmt.Errorf("登录成功但未获取到 token")
	}

	return token, nil
}

// CreateDir 创建远程目录
func (c *ZimaOSClient) CreateDir(baseURL, token, dirPath string) error {
	payload := map[string]string{"path": dirPath}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", baseURL+"/v2_1/files/folder", bytes.NewReader(body))
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("创建目录请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("404: API 不支持")
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// 忽略目录已存在的错误
	if err := c.assertResponse(resp.StatusCode, result, "创建目录"); err != nil {
		if c.isDirExistsError(err.Error()) {
			return nil
		}
		return err
	}

	return nil
}

// UploadFile 上传文件到 ZimaOS
func (c *ZimaOSClient) UploadFile(baseURL, token, remoteDir, filename, localPath string) error {
	stat, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// 添加 path 字段
	writer.WriteField("path", remoteDir)

	// 添加 modTime 字段
	modTime := stat.ModTime().Unix()
	writer.WriteField("modTime", fmt.Sprintf("%d", modTime))

	// 添加文件
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("创建表单文件失败: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("写入文件内容失败: %w", err)
	}

	writer.Close()

	req, _ := http.NewRequest("POST", baseURL+"/v2_1/files/file/uploadV2", &buf)
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// 上传可能需要更长时间
	uploadClient := &http.Client{Timeout: 5 * time.Minute}
	resp, err := uploadClient.Do(req)
	if err != nil {
		return fmt.Errorf("上传请求失败: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	return c.assertResponse(resp.StatusCode, result, "上传")
}

// extractToken 从响应中提取 token
func (c *ZimaOSClient) extractToken(data map[string]interface{}) string {
	// 尝试多种路径提取 token: data.token.access_token / data.token / token
	if d, ok := data["data"].(map[string]interface{}); ok {
		if t, ok := d["token"].(map[string]interface{}); ok {
			if at, ok := t["access_token"].(string); ok {
				return at
			}
		}
		if t, ok := d["token"].(string); ok {
			return t
		}
	}
	if t, ok := data["token"].(string); ok {
		return t
	}
	return ""
}

// assertResponse 检查响应是否成功
func (c *ZimaOSClient) assertResponse(statusCode int, data map[string]interface{}, action string) error {
	if statusCode < 200 || statusCode >= 300 {
		msg := c.extractMessage(data)
		if msg == "" {
			msg = fmt.Sprintf("%s失败(%d)", action, statusCode)
		}
		return fmt.Errorf("%s", msg)
	}

	// 检查 success 字段
	if success, ok := data["success"]; ok {
		if s, ok := success.(bool); ok && !s {
			return fmt.Errorf("%s", c.extractMessage(data))
		}
		if s, ok := success.(float64); ok && s != 200 {
			return fmt.Errorf("%s", c.extractMessage(data))
		}
	}

	// 检查 code 字段
	if code, ok := data["code"].(float64); ok && code != 200 {
		return fmt.Errorf("%s", c.extractMessage(data))
	}

	return nil
}

// extractMessage 从响应中提取错误消息
func (c *ZimaOSClient) extractMessage(data map[string]interface{}) string {
	if msg, ok := data["message"].(string); ok && msg != "" {
		return msg
	}
	if msg, ok := data["msg"].(string); ok && msg != "" {
		return msg
	}
	return ""
}

// isDirExistsError 检查是否是目录已存在的错误
func (c *ZimaOSClient) isDirExistsError(msg string) bool {
	keywords := []string{"exist", "exists", "已存在", "already"}
	for _, kw := range keywords {
		if bytes.Contains([]byte(msg), []byte(kw)) {
			return true
		}
	}
	return false
}
