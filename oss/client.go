package oss

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config OSS 配置
type Config struct {
	BaseURL string // 文件访问的前缀 URL，为空时返回相对路径
	BaseDir string // 上传的前缀文件夹,不能为空
}

// Client OSS 客户端。
type Client struct {
	baseURL string
	baseDir string
}

func New(cfg Config) (*Client, error) {
	if err := os.MkdirAll(cfg.BaseDir, 0755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %v", err)
	}
	return &Client{
		baseURL: cfg.BaseURL,
		baseDir: cfg.BaseDir,
	}, nil
}

// 保存文件
func (c *Client) SaveFile(path string, suffix, b64 string) (string, error) {

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

// 获取文件url
func (c *Client) GetFileUrl(path, name string) string {
	if name == "" {
		return ""
	}
	return c.baseURL + "/" + c.baseDir + "/" + path + "/" + strings.Split(name, "_")[0] + "/" + name
}

// 删除文件
func (c *Client) DelFile(path, name string) {
	if name == "" {
		return
	}

	filePath := filepath.Join(c.baseDir, path, strings.Split(name, "_")[0], name)

	os.Remove(filePath)
}
