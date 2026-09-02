package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"sync"

	"github.com/golang-jwt/jwt/v5"
)

// 密钥以明文形式存在于源码中，可通过 SetPrivateKey / SetPublicKey 在运行时替换。
// RSA 公钥/私钥为 PKCS#1 或 PKCS#8 / X.509 格式的 PEM 字符串。

var (
	mu = &sync.RWMutex{} // 保护密钥，避免并发替换导致 data race

	privateKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQCPTgabHgJJHuIM
Kb5X9fWTOGD4Y0kXkEpuQ6sMt7TI7gIqstvuCDnQ+TfA2g708fW6RF/XUMT80r9B
j7I9FvMJt9MzGpy1lUktNh5oKgQTR+NlUiCnFtYZWb9CONa7YymKbD/wAt9VBRH9
FkNM7kLn7tL5S1/NZbFMqtvwHC1PxyhLoT0nLFWDVUsXYOOT7jywOuKKQLNgGwvJ
CGeH1jJ+CLdELTcNV7gZqrcUrxKEwEYI12Y1rI62OUMvwwQ0oAWn/dlrpLcgLP/z
xCJ3kpGGdiZFcBmd/TUgmR7r23A5Wp2IU2Vgf7A1omdcq9DBl98CAgskm4JxzNmi
F6PDjp0XAgMBAAECggEACEqrdy41U6XFzo5bxRsmKm6IrdaQ1Bw1MkwYCZRXkYiz
92SB9TPkpILHBxGW6/VUEoMCSKMTws0u48w8s+wwA8/vGHXhu1/36/XFrKFBuxvd
vG8UFJbtrGnU9y/yvMTwEmJREMIZygGRGOPA4SKoHGNlMad605eeuqDoOOxocsUs
5/GfQl0djTMPDVHbEdloPYEmn3c79eHBSvs6J14UhyoG+cNjesrxLMqiqYchHE3X
ccGeSXUVaNcrJUgmYUH1RswDBgDwOmJEqd4wp+lXClplZZKz8WFisUnRpDxBtaDe
U2onLsAU6jJd7tAOU+u1FKuYTvWEH5KZ0oQMQ7PBgQKBgQDZYJ7jn49wrIltFZlm
PqOsdEs2EqeJMdvTa7RIKF/Dl57HMLYIaZXlt1gVOFz7KKbuZzVg8/DYSNlJPSaY
+PiyyOVVLdUUmwcrLGLMa6iSlLAOUWXppom5KxYhYqi8gTwGah9a9uR068THkw3Q
7j9cdl8lY3TnCeTcYLQV38eP/QKBgQCoxDjzuYud7oTUQocxeKJ0w2dLNdpInr0d
lKby6SsZNnGf4z0KCWZHmbG5V+EJFfVxlSOjSzorOcIAqxjuYOWISfIMNOb6VF8k
+OAV55f5S1SxVt5Wk7dCh5HqOWhPIuHVeoZ/jMLzTbHeW5NkA0aemraLY1KvleZG
XIlxSDtbowKBgFT0cwSI3plthQQR9fLEtlj21lIatkljKAOXy0yMIukhP5efjPT6
tu+hWRZqAcTS3XK8+Vqb29vblLgP4x7T5vaQlzhUAjvcXs/bt/0mcipfW/MsksTf
JmIs2ahQk5ugcmIbZYe6iAy9/Bj3euXVxwOO656EITMOZdhPHvKRk7/NAoGBAJrR
Js2ucH24yPFO9mZTm/QxLRi5ljz4IdR5AY3kiDzgzOs3sk76wHD+dSLpku6azkYb
4k1yPTJaEbY7Puuux+F2tCyuexU5QO7Rv/9YLPnsOQ1V+zDA4WSOqTSM5TtzbGhB
thBcOJqps3mf2F3vA2GL29mSi8+3Wz9AtHTPJmSxAoGBAMWBE77owY87WAu5W/Fm
tSDBI7WwVhTM5+paCn2RFa7S75Ey6fLScGD0dJsabg9HlxBUlXHtJufOL79gkgkl
6jCMOV7qkWhu/Z+V/tWuaVg+nNyEiPtRf6jpyG3GMADp/99hmE4spDktwcnVkPy/
UCjWHEGv/KOf6DCTAlI3f5oj
-----END RSA PRIVATE KEY-----
`

	publicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAj04Gmx4CSR7iDCm+V/X1
kzhg+GNJF5BKbkOrDLe0yO4CKrLb7gg50Pk3wNoO9PH1ukRf11DE/NK/QY+yPRbz
CbfTMxqctZVJLTYeaCoEE0fjZVIgpxbWGVm/QjjWu2Mpimw/8ALfVQUR/RZDTO5C
5+7S+UtfzWWxTKrb8BwtT8coS6E9JyxVg1VLF2Djk+48sDriikCzYBsLyQhnh9Yy
fgi3RC03DVe4Gaq3FK8ShMBGCNdmNayOtjlDL8MENKAFp/3Za6S3ICz/88Qid5KR
hnYmRXAZnf01IJke69twOVqdiFNlYH+wNaJnXKvQwZffAgILJJuCcczZohejw46d
FwIDAQAB
-----END PUBLIC KEY-----`

	signingMethod = jwt.SigningMethodRS256 // RS256 / RS384 / RS512
)

// SetPrivateKey 用 PEM 字符串替换私钥。内部会立即解析校验，非法则返回错误并保持原值。
func SetPrivateKey(pemStr string) error {
	key, err := parseRSAPrivateKey([]byte(pemStr))
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}
	mu.Lock()
	defer mu.Unlock()
	privateKeyPEM = pemStr
	_ = key
	return nil
}

// SetPublicKey 用 PEM 字符串替换公钥。内部会立即解析校验，非法则返回错误并保持原值。
func SetPublicKey(pemStr string) error {
	if _, err := parseRSAPublicKey([]byte(pemStr)); err != nil {
		return fmt.Errorf("parse public key: %w", err)
	}
	mu.Lock()
	defer mu.Unlock()
	publicKeyPEM = pemStr
	return nil
}

type Claims[T any] struct {
	jwt.RegisteredClaims
	Meta T `json:"meta"`
}

// GenerateToken 使用 RSA 私钥签名生成 token。
func GenerateToken[T any](claims Claims[T]) (string, error) {
	key, err := currentPrivateKey()
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(signingMethod, claims)
	return token.SignedString(key)
}

// ParseToken 使用 RSA 公钥验签并解析 token。
func ParseToken[T any](tokenString string) (*Claims[T], error) {
	key, err := currentPublicKey()
	if err != nil {
		return nil, err
	}

	claims := &Claims[T]{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			// 强制校验签名算法必须是指定 RSA 算法（防 alg 混淆攻击）
			if token.Method != signingMethod {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return key, nil
		},
		jwt.WithValidMethods([]string{signingMethod.Alg()}), // 额外一层算法白名单防护
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// ParseWithClaims 成功时 token.Claims 即传入的 claims 实例
	return claims, nil
}

// ---- 内部工具 ----

// currentPrivateKey 返回当前私钥的 RSA 私钥对象。
func currentPrivateKey() (*rsa.PrivateKey, error) {
	mu.RLock()
	defer mu.RUnlock()
	key, err := parseRSAPrivateKey([]byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse current private key: %w", err)
	}
	return key, nil
}

// currentPublicKey 返回当前公钥的 RSA 公钥对象。
func currentPublicKey() (*rsa.PublicKey, error) {
	mu.RLock()
	defer mu.RUnlock()
	key, err := parseRSAPublicKey([]byte(publicKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("parse current public key: %w", err)
	}
	return key, nil
}

// parseRSAPrivateKey 兼容 PKCS#1 与 PKCS#8 格式解析私钥。
func parseRSAPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid private key PEM")
	}

	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rsa, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
		return rsa, nil
	}
	return nil, errors.New("unsupported private key format")
}

// parseRSAPublicKey 兼容 PKIX / X.509 SubjectPublicKeyInfo 与 PKCS#1 格式解析公钥。
func parseRSAPublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid public key PEM")
	}

	if k, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsa, ok := k.(*rsa.PublicKey); ok {
			return rsa, nil
		}
		return nil, errors.New("public key is not RSA")
	}
	if k, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, errors.New("unsupported public key format")
}
