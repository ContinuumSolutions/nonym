package auth

import (
	"time"

	gotp "github.com/pquerna/otp/totp"
)

func totpGenerateCodeAt(secret string, t time.Time) (string, error) {
	return gotp.GenerateCodeCustom(secret, t, gotp.ValidateOpts{
		Period:    totpPeriod,
		Skew:      0,
		Digits:    totpDigits,
		Algorithm: totpAlgorithm,
	})
}
