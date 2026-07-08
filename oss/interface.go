package oss

// Storage OSS 存储接口。
type Storage interface {
	SaveFile(path string, suffix string, b64 string) (string, error)
	GetFileUrl(path string, name string) string
	DelFile(path string, name string)
}
