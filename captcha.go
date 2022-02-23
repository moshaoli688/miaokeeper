package main

import (
	"bytes"
	"math/rand"

	"github.com/steambap/captcha"
)

type CaptchaType uint

const (
	CaptchaTypeNormal CaptchaType = iota
	CaptchaTypeMathExpr
)

func GenerateRandomCaptcha() (*bytes.Buffer, []string) {
	if rand.Intn(10) >= 5 {
		buf, result := GenerateCaptcha(CaptchaTypeMathExpr)
		return buf, GenerateCaptchaOptions(CaptchaTypeMathExpr, result)
	}
	buf, result := GenerateCaptcha(CaptchaTypeNormal)
	return buf, GenerateCaptchaOptions(CaptchaTypeNormal, result)
}

func GenerateCaptcha(captchaType CaptchaType) (*bytes.Buffer, string) {
	var data *captcha.Data = nil

	w := 280
	h := w * 2 / 5
	if captchaType == CaptchaTypeNormal {
		data, _ = captcha.New(w, h)
	} else {
		data, _ = captcha.NewMathExpr(w, h)
	}

	buf := bytes.Buffer{}
	data.WriteImage(&buf)
	return &buf, data.Text
}

var ObfsExcludes = [][]string{
	{"O", "0", "Q", "D", "o"},
	{"1", "I", "l"},
	{"2", "z", "Z"},
}
var ObfsSet = []rune("1234567890qwertyuiopasdfghjklzxcvbnmQWERTYUIOPASDFGHJKLZXCVBNM")
var ObfsSetNum = []rune("1234567890")

func GenerateCaptchaRandomize(captchaType CaptchaType) rune {
	if captchaType == CaptchaTypeMathExpr {
		return ObfsSetNum[rand.Intn(len(ObfsSetNum))]
	}
	return ObfsSet[rand.Intn(len(ObfsSet))]
}

func GenerateCaptchaReplacableChar(captchaType CaptchaType, s rune) rune {
	excludes := []string{}
	for _, ex := range ObfsExcludes {
		if ContainsString(ex, string(s)) {
			excludes = ex
		}
	}

	attempts := 0
	for {
		attempts += 1
		c := GenerateCaptchaRandomize(captchaType)
		if !ContainsString(excludes, string(c)) && c != s {
			return c
		}
		if attempts > 10 {
			return '5'
		}
	}
}

func GenerateCaptchaOptions(captchaType CaptchaType, original string) []string {
	waitList := []string{original}
	options := 4
	if captchaType == CaptchaTypeMathExpr {
		options = 6
	}
	for i := 0; i < options-1; i++ {
		attempts := 2
		target := []rune(original)
		if captchaType == CaptchaTypeMathExpr {
			attempts = 1
			if len(target) > 1 {
				attempts = 2
			}
		}
		for j := 0; j < attempts; j += 1 {
			index := rand.Intn(len(target))
			target[index] = GenerateCaptchaReplacableChar(captchaType, target[index])
		}
		waitList = append(waitList, string(target))
	}

	return waitList
}
