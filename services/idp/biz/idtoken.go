package biz

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/castlexu/micro-service/pkg/errno"
)

// decodeIDToken 解码 Google id_token（JWT）的 payload 部分，不验签
// （验签已由 golang.org/x/oauth2 的 Exchange 隐式完成：Google 只在合法 client 的 exchange 中返回有效 id_token）。
func decodeIDToken(rawJWT string) (*GoogleUserInfo, error) {
	parts := strings.Split(rawJWT, ".")
	if len(parts) != 3 {
		return nil, errno.ErrInvalidCredentials.WithMessage("idp: malformed id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errno.ErrInvalidCredentials.WithMessagef("idp: decode id_token payload: %v", err)
	}
	var claims struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, errno.ErrInvalidCredentials.WithMessagef("idp: unmarshal id_token: %v", err)
	}
	if claims.Sub == "" || claims.Email == "" {
		return nil, errno.ErrInvalidCredentials.WithMessage("idp: id_token missing sub or email")
	}
	return &GoogleUserInfo{
		Sub:       claims.Sub,
		Email:     claims.Email,
		Name:      claims.Name,
		AvatarURL: claims.Picture,
	}, nil
}
