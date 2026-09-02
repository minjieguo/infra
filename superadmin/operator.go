package superadmin

import (
	"fmt"

	"github.com/pquerna/otp/totp"
)

var (
	secret = "OZSWY3ZYGM2TM"
)

// SetSecret 设置秘钥
func SetSecret(newSecret string) error {
	if secret == "" {
		return fmt.Errorf("empty secret")
	}
	secret = newSecret
	return nil
}

func Validate(password string) bool {
	return totp.Validate(password, secret)
}
