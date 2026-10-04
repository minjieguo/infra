package oss

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// localClient 本地磁盘存储实现。
type localClient struct {
	baseURL string
	baseDir string
}

// NewLocalClient 创建本地磁盘存储客户端。
//
// BaseDir 会先做路径归一: 反斜杠统一转为正斜杠, 并清理重复的 "/" 与 "."、".."。
// 绝对路径 (如 /var/www/upload) 保留根标志; 归一后的目录以 slash 形式保存,
// 调用系统 API 时通过 filepath.FromSlash 转回当前系统的本地分隔符 (Windows 下为 "\\")。
func NewLocalClient(cfg Config) (Storage, error) {
	baseDir := normalizeSlash(cfg.BaseDir)
	if baseDir == "" {
		return nil, fmt.Errorf("上传目录不能为空")
	}
	if err := os.MkdirAll(filepath.FromSlash(baseDir), 0755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %v", err)
	}
	return &localClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		baseDir: baseDir,
	}, nil
}

// SaveFile 保存文件到本地磁盘, 返回生成的文件名。
func (c *localClient) SaveFile(path string, suffix, b64 string) (string, error) {

	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("解码失败: %v\n", err)
	}

	monthPath := time.Now().Format("200601")
	fileName := fmt.Sprintf("%s_%d.%s", monthPath, time.Now().UnixNano(), suffix)

	directory := filepath.Join(c.baseDir, path, monthPath)

	// 检查目录是否存在，不存在则创建
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		err := os.MkdirAll(directory, 0755)
		if err != nil {
			return "", fmt.Errorf("创建目录失败: %v", err)
		}
	}

	filePath := filepath.Join(directory, fileName)

	// 写入文件
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", fmt.Errorf("写入文件失败: %v", err)
	}

	return fileName, nil
}

// GetFileUrl 获取文件访问地址。
func (c *localClient) GetFileUrl(path, name string) string {
	if name == "" {
		return ""
	}
	// baseDir 以 slash 形式保存, URL 统一使用 "/" 拼接, 并去除拼接产生的重复斜杠。
	dir := strings.Trim(c.baseDir, "/")
	if dir != "" {
		dir += "/"
	}
	return c.baseURL + "/" + dir + normalizeSlash(path) + "/" + strings.Split(name, "_")[0] + "/" + name
}

// DelFile 删除本地文件。
func (c *localClient) DelFile(path, name string) {
	if name == "" {
		return
	}

	filePath := filepath.Join(c.baseDir, path, strings.Split(name, "_")[0], name)

	os.Remove(filePath)
}
