package service

import (
	"testing"
)

func Test_fetchCaptcha(t *testing.T) {
	captcha, err := fetchCaptcha()
	if err != nil {
		t.Fail()
	}

	t.Logf("captcha is : %#v\n", captcha)
}
