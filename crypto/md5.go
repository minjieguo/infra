package crypto

import (
	"crypto/md5"
	"encoding/hex"
)

// MD5String 计算字符串的 MD5 哈希值
func MD5String(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}
