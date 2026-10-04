package oss

import "github.com/minjieguo/infra/logger"

// Config OSS 配置。
type Config struct {
	Mode     string        // 存储模式: local / aliyun, 默认为 local
	BaseURL  string        // 文件访问的前缀 URL, 为空时返回相对路径
	BaseDir  string        // 上传的前缀文件夹; 本地模式必填, 阿里云模式为可选 key 前缀
	Endpoint string        // 阿里云 OSS Endpoint, 如 oss-cn-hangzhou.aliyuncs.com
	Region   string        // 阿里云 Region, 如 cn-hangzhou, 可空
	Bucket   string        // 阿里云 OSS Bucket 名称
	AccessID string        // 阿里云 AccessKey ID
	Secret   string        // 阿里云 AccessKey Secret
	Logger   logger.Logger // 日志, 可为空
}

// Storage OSS 存储接口。
type Storage interface {
	SaveFile(path string, suffix string, b64 string) (string, error)
	GetFileUrl(path string, name string) string
	DelFile(path string, name string)
}
