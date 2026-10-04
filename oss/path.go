package oss

import (
	"path"
	"strings"
)

// normalizeSlash 归一化路径中的分隔符与冗余层级。
//
// 处理规则:
//   - 反斜杠 "\" 统一转为正斜杠 "/", 兼容 Windows 风格配置;
//   - 通过 path.Clean 去掉重复的 "/" 以及 "."、".." 层级;
//   - 结果保留根路径标志: 入参为绝对路径 (以 "/" 开头) 时, 返回值仍以 "/" 开头,
//     避免本地模式下绝对路径被当成相对目录; 相对路径则不含首尾 "/"。
//
// 本地模式与阿里云模式共用该函数: 前者在调用系统 API 前转回系统分隔符,
// 后者用于 object key 前缀（key 不以 "/" 开头, 需由调用方再 TrimLeft）。
func normalizeSlash(p string) string {
	rooted := strings.HasPrefix(strings.ReplaceAll(p, "\\", "/"), "/")

	p = strings.ReplaceAll(p, "\\", "/")
	p = path.Clean(p)
	if p == "." || p == "/" {
		return ""
	}

	p = strings.Trim(p, "/")
	if p == "" {
		return ""
	}
	if rooted {
		return "/" + p
	}
	return p
}

// trimScheme 去除 Endpoint 中的协议前缀与尾部斜杠, 返回纯域名。
// 例: https://oss-cn-hangzhou.aliyuncs.com/ -> oss-cn-hangzhou.aliyuncs.com
func trimScheme(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if idx := strings.Index(endpoint, "://"); idx >= 0 {
		endpoint = endpoint[idx+3:]
	}
	return strings.Trim(endpoint, "/")
}
