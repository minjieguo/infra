package oss

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/minjieguo/infra/internal/testenv"
)

// 测试用阿里云 OSS 配置, 从仓库根目录的 .env 读取;
// 未配置时跳过需要真实 OSS 的测试。
//
// .env 示例：
//
//	OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com
//	OSS_REGION=cn-hangzhou
//	OSS_BUCKET=your-bucket
//	OSS_ACCESS_ID=your-access-key-id
//	OSS_ACCESS_SECRET=your-access-key-secret
//	OSS_BASE_URL=            # 可选, 文件访问前缀(CDN/自定义域名)
//	OSS_BASE_DIR=infra-test
func aliyunTestConfig(t *testing.T) Config {
	t.Helper()

	endpoint := testenv.Lookup("OSS_ENDPOINT")
	bucket := testenv.Lookup("OSS_BUCKET")
	accessID := testenv.Lookup("OSS_ACCESS_ID")
	secret := testenv.Lookup("OSS_ACCESS_SECRET")

	if endpoint == "" || bucket == "" || accessID == "" || secret == "" {
		t.Skip("未配置 OSS_ENDPOINT/OSS_BUCKET/OSS_ACCESS_ID/OSS_ACCESS_SECRET(参考 .env), 跳过阿里云 OSS 集成测试")
	}

	return Config{
		Mode:     ModeAliyun,
		Endpoint: endpoint,
		Region:   testenv.Lookup("OSS_REGION"),
		Bucket:   bucket,
		AccessID: accessID,
		Secret:   secret,
		// 文件访问前缀, 未配置时 GetFileUrl 回退为虚拟主机风格域名
		BaseURL: testenv.Lookup("OSS_BASE_URL"),
		// 使用独立前缀, 避免污染业务目录
		BaseDir: testenv.Get("OSS_BASE_DIR", "infra-test"),
	}
}

// newAliyunClient 按 .env 配置创建阿里云 OSS 客户端, 未配置时跳过测试。
func newAliyunClient(t *testing.T) Storage {
	t.Helper()

	client, err := NewAliyunClient(aliyunTestConfig(t))
	if err != nil {
		t.Fatalf("创建阿里云 OSS 客户端失败: %v", err)
	}
	return client
}

// newTestImage 代码生成一张 PNG 图片, 返回 base64 编码内容。
//
// 像素按渐变规则确定性生成, 便于复现; 使用标准库 image/png, 不引入额外依赖。
func newTestImage(t *testing.T, width, height int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			// 渐变色彩, 避免整图单色被压缩成极小体积
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / max(width-1, 1)),
				G: uint8(y * 255 / max(height-1, 1)),
				B: 128,
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("生成 PNG 图片失败: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// TestAliyunSaveFile 上传代码生成的图片并校验返回的文件名规则。
//
// 运行前需在 .env 配置真实 OSS 参数, 未配置时自动跳过。
func TestAliyunSaveFile(t *testing.T) {
	client := newAliyunClient(t)

	// 上传内容为代码生成的 PNG 图片
	name, err := client.SaveFile("test", "png", newTestImage(t, 64, 64))
	if err != nil {
		t.Fatalf("SaveFile 失败: %v", err)
	}
	if name == "" {
		t.Fatal("SaveFile 返回的文件名为空")
	}
	// 文件名规则: {年月}_{UnixNano}.{suffix}, 与本地模式一致
	if !strings.HasSuffix(name, ".png") {
		t.Errorf("文件名后缀不符合预期: %s", name)
	}
	if len(strings.Split(name, "_")) != 2 {
		t.Errorf("文件名格式不符合 {年月}_{UnixNano}.{suffix}: %s", name)
	}

	// 上传后可生成访问地址
	url := client.GetFileUrl("test", name)
	if url == "" {
		t.Error("GetFileUrl 返回空地址")
	}
	t.Logf("已上传图片, 地址: %s", url)

	t.Cleanup(func() { client.DelFile("test", name) })
}

// TestLocalSaveImage 本地模式上传代码生成的图片, 校验图片内容合法。
//
// 不依赖外部服务, 始终执行。
func TestLocalSaveImage(t *testing.T) {
	client, err := NewLocalClient(Config{BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("创建本地客户端失败: %v", err)
	}

	b64 := newTestImage(t, 32, 32)
	name, err := client.SaveFile("test", "png", b64)
	if err != nil {
		t.Fatalf("SaveFile 失败: %v", err)
	}

	// 校验生成的图片可被解码为合法 PNG
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("图片解码失败: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("生成的图片内容为空")
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		t.Fatalf("生成的图片不是合法 PNG: %v", err)
	}

	t.Logf("本地图片: %s (name=%s, size=%s)", client.GetFileUrl("test", name), name, fmt.Sprintf("%dB", len(raw)))

	client.DelFile("test", name)
}

// TestAliyunDelFile 上传代码生成的图片后删除, 校验删除流程不报错。
//
// 运行前需在 .env 配置真实 OSS 参数, 未配置时自动跳过。
func TestAliyunDelFile(t *testing.T) {
	client := newAliyunClient(t)

	name, err := client.SaveFile("test", "png", newTestImage(t, 16, 16))
	if err != nil {
		t.Fatalf("SaveFile 失败: %v", err)
	}

	// DelFile 无返回值, 删除失败仅记日志; 此处校验不 panic
	client.DelFile("test", name)

	// 重复删除也应安全
	client.DelFile("test", name)
}

// TestAliyunSaveFileInvalidBase64 校验非法 base64 输入返回错误。
//
// 该用例不依赖真实 OSS 网络, 但仍需完成配置校验, 故同样在未配置时跳过。
func TestAliyunSaveFileInvalidBase64(t *testing.T) {
	client := newAliyunClient(t)

	if _, err := client.SaveFile("test", "txt", "!!!not-base64!!!"); err == nil {
		t.Error("非法 base64 输入应当返回错误")
	}
}

// TestLocalSaveAndDelete 本地模式上传/取址/删除全流程。
//
// 本地模式不依赖外部服务, 始终执行。
func TestLocalSaveAndDelete(t *testing.T) {
	client, err := NewLocalClient(Config{BaseURL: "https://cdn.example.com/", BaseDir: t.TempDir()})
	if err != nil {
		t.Fatalf("创建本地客户端失败: %v", err)
	}

	name, err := client.SaveFile("test", "txt", base64.StdEncoding.EncodeToString([]byte("hello local")))
	if err != nil {
		t.Fatalf("SaveFile 失败: %v", err)
	}

	// BaseURL 尾部斜杠应被裁剪, 拼接结果不含重复斜杠
	url := client.GetFileUrl("test", name)
	if strings.Contains(url, "//"+strings.Split(name, "_")[0]) || !strings.HasPrefix(url, "https://cdn.example.com/") {
		t.Errorf("GetFileUrl 结果不符合预期: %s", url)
	}

	client.DelFile("test", name)

	if url := client.GetFileUrl("test", ""); url != "" {
		t.Errorf("name 为空时应返回空地址, 实际: %s", url)
	}
}

// TestNormalizeSlash 校验路径分隔符归一逻辑。
func TestNormalizeSlash(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{".", ""},
		{"/", ""},
		{"//", ""},
		{"upload", "upload"},
		{"/upload/", "/upload"},
		{"upload//2026/", "upload/2026"},
		{`upload\2026\09`, "upload/2026/09"},
		{"a/../b", "b"},
		// 绝对路径保留根标志, 避免本地模式写到相对目录
		{"/var/www/upload", "/var/www/upload"},
	}

	for _, c := range cases {
		if got := normalizeSlash(c.in); got != c.want {
			t.Errorf("normalizeSlash(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

// TestTrimScheme 校验 Endpoint 协议前缀剥离逻辑。
func TestTrimScheme(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"oss-cn-hangzhou.aliyuncs.com", "oss-cn-hangzhou.aliyuncs.com"},
		{"https://oss-cn-hangzhou.aliyuncs.com", "oss-cn-hangzhou.aliyuncs.com"},
		{"https://oss-cn-hangzhou.aliyuncs.com/", "oss-cn-hangzhou.aliyuncs.com"},
		{" http://oss-cn-hangzhou.aliyuncs.com ", "oss-cn-hangzhou.aliyuncs.com"},
	}

	for _, c := range cases {
		if got := trimScheme(c.in); got != c.want {
			t.Errorf("trimScheme(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

// TestAliyunObjectKey BaseDir 归一与 object key 拼接。
//
// 不依赖真实 OSS 网络, 仅构造客户端校验 key 规则, 因此不受配置缺失影响。
func TestAliyunObjectKey(t *testing.T) {
	cases := []struct {
		name    string
		baseDir string
		path    string
		file    string
		want    string
	}{
		{"普通前缀", "upload", "avatar", "202601_123.png", "upload/avatar/202601/202601_123.png"},
		// 绝对路径与尾部斜杠应被归一, 不产生重复或开头斜杠
		{"绝对路径前缀", "/upload/photos/", "avatar", "202601_123.png", "upload/photos/avatar/202601/202601_123.png"},
		{"反斜杠前缀", `upload\photos`, "avatar", "202601_123.png", "upload/photos/avatar/202601/202601_123.png"},
		{"空前缀", "", "avatar", "202601_123.png", "avatar/202601/202601_123.png"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := &aliyunClient{
				baseDir: strings.TrimLeft(normalizeSlash(c.baseDir), "/"),
			}
			if got := client.objectKey(c.path, c.file); got != c.want {
				t.Errorf("objectKey(%q, %q) = %q, 期望 %q", c.path, c.file, got, c.want)
			}
		})
	}
}
