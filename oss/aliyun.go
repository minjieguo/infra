package oss

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/minjieguo/infra/logger"
	"go.uber.org/zap"
)

// aliyunOpTimeout 阿里云单次操作超时, 避免无超时裸调用。
const aliyunOpTimeout = 30 * time.Second

// aliyunClient 阿里云 OSS 存储实现, 基于官方 SDK alibabacloud-oss-go-sdk-v2。
//
// BaseDir 约定: 阿里云 OSS 的 Bucket 为扁平 key-value 结构, 不存在真实目录,
// 因此 BaseDir 作为全局前缀参与 object key 拼接, 与本地模式语义保持一致:
//
//	key = BaseDir + "/" + path + "/" + 年月 + "/" + fileName
//
// 说明:
//   - BaseDir 允许为空, 表示直接存放于 Bucket 根目录, 故不参与配置必填校验;
//   - 构造函数已对 BaseDir 做归一化, 客户端的 baseDir 字段已转为 "/" 且不含首尾斜杠
//     (即使传入绝对路径也会去掉根标志), key 拼接时仅需中间加 "/",
//     不会出现 "//" 或 key 以 "/" 开头;
//   - 多环境可共用一个 Bucket, 用不同 BaseDir 前缀做隔离。
//
// 与本地模式的差异: 本地模式 BaseDir 为必填的磁盘目录, 阿里云模式为可选 key 前缀。
type aliyunClient struct {
	client   *oss.Client
	bucket   string        // Bucket 名称
	baseURL  string        // 自定义访问前缀, 为空时按虚拟主机风格域名生成
	baseDir  string        // 已归一化的 key 前缀, 不含首尾 "/"
	endpoint string        // 已去除协议前缀的 Endpoint, 仅用于生成默认访问域名
	logger   logger.Logger // 日志, 可为空
}

// NewAliyunClient 创建阿里云 OSS 存储客户端。
//
// 必填校验项为 Endpoint / Bucket / AccessID / Secret;
// BaseDir 为可选 key 前缀, 不参与校验, 但会做分隔符归一化:
// 反斜杠统一转为 "/", 去掉重复的 "/" 与 "."、".." 层级,
// 并剥离根标志 (OSS 的 object key 不以 "/" 开头)。
func NewAliyunClient(cfg Config) (Storage, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessID == "" || cfg.Secret == "" {
		return nil, fmt.Errorf("阿里云 OSS 配置不完整")
	}

	sdkCfg := oss.Config{
		Endpoint: new(cfg.Endpoint),
		CredentialsProvider: credentials.NewStaticCredentialsProvider(
			cfg.AccessID,
			cfg.Secret,
		),
	}
	if cfg.Region != "" {
		sdkCfg.Region = new(cfg.Region)
	}

	return &aliyunClient{
		client:   oss.NewClient(&sdkCfg),
		bucket:   cfg.Bucket,
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		endpoint: trimScheme(cfg.Endpoint),
		logger:   cfg.Logger,
		// 先归一化, 再剥离可能的根路径标志, 保证是纯 key 前缀。
		baseDir: strings.TrimLeft(normalizeSlash(cfg.BaseDir), "/"),
	}, nil
}

// SaveFile 上传文件到阿里云 OSS, 返回生成的文件名。
//
// 文件名规则与本地模式保持一致: {年月}_{UnixNano}.{suffix}。
func (c *aliyunClient) SaveFile(path string, suffix string, b64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("解码失败: %v\n", err)
	}

	now := time.Now()
	fileName := fmt.Sprintf("%s_%d.%s", now.Format("200601"), now.UnixNano(), suffix)

	ctx, cancel := context.WithTimeout(context.Background(), aliyunOpTimeout)
	defer cancel()

	_, err = c.client.PutObject(ctx, &oss.PutObjectRequest{
		Bucket: oss.Ptr(c.bucket),
		Key:    oss.Ptr(c.objectKey(path, fileName)),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return "", fmt.Errorf("上传文件失败: %v", err)
	}

	return fileName, nil
}

// GetFileUrl 获取阿里云 OSS 文件访问地址。
//
// 地址前缀优先使用配置的 BaseURL, 未配置时回退为虚拟主机风格域名
// https://{Bucket}.{Endpoint}/{BaseDir}。
func (c *aliyunClient) GetFileUrl(path string, name string) string {
	if name == "" {
		return ""
	}

	key := c.objectKey(path, name)
	if c.baseURL != "" {
		return c.baseURL + "/" + key
	}
	return "https://" + c.bucket + "." + c.endpoint + "/" + key
}

// DelFile 删除阿里云 OSS 文件。
//
// 与本地模式的 os.Remove 一致, 删除失败不向调用方返回错误, 由日志侧排查。
func (c *aliyunClient) DelFile(path string, name string) {
	if name == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), aliyunOpTimeout)
	defer cancel()

	if _, err := c.client.DeleteObject(ctx, &oss.DeleteObjectRequest{
		Bucket: new(c.bucket),
		Key:    new(c.objectKey(path, name)),
	}); err != nil && c.logger != nil {
		c.logger.Warn("删除 OSS 文件失败",
			zap.String("bucket", c.bucket),
			zap.String("key", c.objectKey(path, name)),
			zap.Error(err),
		)
	}
}

// objectKey 按统一规则拼接 object key, 供上传、取地址、删除复用。
//
// 各段之间以 "/" 连接, 空段自动跳过, 保证不会出现重复斜杠。
func (c *aliyunClient) objectKey(path, name string) string {
	segments := make([]string, 0, 4)
	if c.baseDir != "" {
		segments = append(segments, c.baseDir)
	}
	if p := normalizeSlash(path); p != "" {
		segments = append(segments, p)
	}
	// 文件名前缀为年月, 与本地模式保持一致, 便于反推目录层级。
	if n := strings.ReplaceAll(name, "\\", "/"); n != "" {
		segments = append(segments, strings.Split(n, "_")[0], n)
	}
	return strings.Join(segments, "/")
}
