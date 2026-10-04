package oss

// 存储模式常量。
const (
	ModeLocal  = "local"  // 本地磁盘存储
	ModeAliyun = "aliyun" // 阿里云 OSS 存储
)

// New 按配置创建 OSS 客户端, 未指定模式时默认使用本地模式。
func New(cfg Config) (Storage, error) {
	if cfg.Mode == ModeAliyun {
		return NewAliyunClient(cfg)
	}
	return NewLocalClient(cfg)
}
