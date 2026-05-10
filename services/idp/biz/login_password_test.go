package biz

import (
	"testing"
)

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		password string
		wantErr  bool
	}{
		{"Sh0rt!", true},              // 长度不足（5位）
		{"alllowercase1!", true},     // 缺大写
		{"ALLUPPERCASE1!", true},     // 缺小写
		{"NoDigitsHere!", true},      // 缺数字
		{"NoSpecial123", true},       // 缺特殊字符
		{"Valid1Pass!", false},       // 合法
		{"Abcdefg1@", false},        // 合法
	}
	for _, c := range cases {
		err := validatePassword(c.password)
		if (err != nil) != c.wantErr {
			t.Errorf("validatePassword(%q) err=%v, wantErr=%v", c.password, err, c.wantErr)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	cases := []struct {
		email   string
		wantErr bool
	}{
		{"user@example.com", false},
		{"user+tag@sub.domain.io", false},
		{"notanemail", true},
		{"missing@domain", true},
		{"@nodomain.com", true},
		{"", true},
	}
	for _, c := range cases {
		err := validateEmail(c.email)
		if (err != nil) != c.wantErr {
			t.Errorf("validateEmail(%q) err=%v, wantErr=%v", c.email, err, c.wantErr)
		}
	}
}
