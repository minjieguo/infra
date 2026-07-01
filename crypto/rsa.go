package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
)

// GenerateRSAKeyPair 生成RSA密钥对
func GenerateRSAKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, &privateKey.PublicKey, nil
}

// RSAEncrypt 使用 RSA 公钥加密明文，返回 Base64 编码的密文
// 使用 OAEP 填充，SHA-256 哈希
func RSAEncrypt(plaintext string, publicKey *rsa.PublicKey) (string, error) {
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, []byte(plaintext), nil)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// RSADecrypt 使用 RSA 私钥解密 Base64 编码的密文，返回明文
// 使用 OAEP 填充，SHA-256 哈希
func RSADecrypt(cipherBase64 string, privateKey *rsa.PrivateKey) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cipherBase64)
	if err != nil {
		return nil, err
	}
	plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// RSASign 使用 RSA 私钥对数据进行签名，返回 Base64 编码的签名
// 使用 PSS 填充，SHA-256 哈希
func RSASign(data []byte, privateKey *rsa.PrivateKey) (string, error) {
	hash := sha256.Sum256(data)
	signature, err := rsa.SignPSS(rand.Reader, privateKey, crypto.SHA256, hash[:], nil)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

// RSAVerify 使用 RSA 公钥验证签名
// signatureBase64 为 Base64 编码的签名，data 为原始数据
func RSAVerify(data []byte, signatureBase64 string, publicKey *rsa.PublicKey) error {
	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(data)
	return rsa.VerifyPSS(publicKey, crypto.SHA256, hash[:], signature, nil)
}

// 公钥导出为 PEM 字符串 PKIX
func PublicKeyToPEMString(publicKey *rsa.PublicKey) (string, error) {
	// 将公钥转换为 DER 格式
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}

	// 创建 PEM Block
	pemBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	// 编码为 PEM 字符串
	pemString := string(pem.EncodeToMemory(pemBlock))
	return pemString, nil
}

// 私钥导出为 PEM 字符串
func PrivateKeyToPEMString(privateKey *rsa.PrivateKey) (string, error) {
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", err
	}
	pemBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	return string(pem.EncodeToMemory(pemBlock)), nil
}

// ParsePublicKeyFromPEM 从 PEM 字符串解析 RSA 公钥
// 支持 "PUBLIC KEY"（PKIX）格式
func ParsePublicKeyFromPEM(pemString string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemString))
	if block == nil {
		return nil, errors.New("无法解析 PEM 数据")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("不是 RSA 公钥")
	}
	return publicKey, nil
}

// ParsePrivateKeyFromPEM 从 PEM 字符串解析 RSA 私钥
// 支持 "RSA PRIVATE KEY"（PKCS#1）格式
func ParsePrivateKeyFromPEM(pemString string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemString))
	if block == nil {
		return nil, errors.New("无法解析 PEM 数据")
	}
	priv, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	privateKey, ok := priv.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("不是 RSA 私钥")
	}
	return privateKey, nil
}
