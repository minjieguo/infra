// Package testenv 为集成测试提供统一的配置加载能力。
//
// 约定：仓库根目录下的 .env 存放集成测试所需的连接信息
// （如 Redis、Kafka 地址）。测试通过 Lookup 系列函数读取配置，
// 未配置时调用方自行决定跳过还是失败。
//
// .env 中的键值会覆盖真实的进程环境变量，
// 因此修改 .env 即可切换测试目标环境。
package testenv

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const envFile = ".env"

var (
	loadOnce sync.Once
	values   map[string]string
)

// loadEnvFile 加载仓库根目录的 .env，进程生命周期内只执行一次。
//
// 测试进程的工作目录是被测包所在目录（如 cache/），
// 因此需要逐级向上查找 .env，直到文件系统根目录。
func loadEnvFile() {
	values = make(map[string]string)

	dir, err := os.Getwd()
	if err != nil {
		return
	}

	for {
		path := filepath.Join(dir, envFile)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			parseEnvFile(path)
			return
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// 已到根目录，未找到 .env
			return
		}
		dir = parent
	}
}

// parseEnvFile 解析 KEY=VALUE 形式的 .env 文件。
//
// 支持空行、以 # 开头的注释行、key 与 value 两侧的空白裁剪，
// 以及单/双引号包裹的 value。解析失败的行会被忽略。
func parseEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		value = strings.TrimSpace(value)
		// 去掉成对的引号，保留引号内的原始内容
		if len(value) >= 2 {
			first, last := value[0], value[len(value)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		values[key] = value
	}
	// 关键：循环结束后检查错误
	if err := scanner.Err(); err != nil {
		// 处理错误，例如返回或记录
		return
	}
}

// Lookup 读取指定 key 的配置。.env 中存在该 key 时以其为准，否则回退到进程环境变量。
func Lookup(key string) string {
	loadOnce.Do(loadEnvFile)

	if value, ok := values[key]; ok && value != "" {
		return value
	}
	return os.Getenv(key)
}

// Has 判断指定 key 是否已配置（值非空）。
func Has(key string) bool {
	return Lookup(key) != ""
}

// Get 读取指定 key 的配置，未配置时返回默认值。
func Get(key string, fallback string) string {
	if value := Lookup(key); value != "" {
		return value
	}
	return fallback
}
