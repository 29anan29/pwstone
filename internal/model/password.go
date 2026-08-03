package model

import (
	"crypto/rand"
	"strings"
	"unicode"
)

const (
	MinMasterLength  = 12
	passwordAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*-_=+"
)

func CheckMasterStrength(pw string) (bool, string) {
	if len(pw) < MinMasterLength {
		return false, "主密码至少 12 位"
	}
	var upper, lower, digit, symbol bool
	for _, c := range pw {
		switch {
		case unicode.IsUpper(c):
			upper = true
		case unicode.IsLower(c):
			lower = true
		case unicode.IsDigit(c):
			digit = true
		case unicode.IsSymbol(c), unicode.IsPunct(c):
			symbol = true
		}
	}
	score := 0
	for _, ok := range []bool{upper, lower, digit, symbol} {
		if ok {
			score++
		}
	}
	if score < 3 {
		return false, "主密码需包含大写、小写、数字、特殊符号中至少 3 类"
	}
	weak := map[string]bool{
		"password123": true, "123456789012": true, "qwertyuiop[]": true,
		"abcdefghijkl": true, "p@ssw0rd1234": true, "admin123456": true,
	}
	if weak[strings.ToLower(pw)] {
		return false, "主密码过于常见，请换一个"
	}
	return true, "强密码"
}

func GeneratePassword(n int) string {
	if n < 8 {
		n = 8
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	out := make([]byte, n)
	for i, v := range b {
		out[i] = passwordAlphabet[int(v)%len(passwordAlphabet)]
	}
	return string(out)
}
