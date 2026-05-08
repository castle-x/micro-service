package main

import (
	"context"
	"errors"

	"github.com/castlexu/micro-service/pkg/errno"
	idpbase "github.com/castlexu/micro-service/services/idp/kitex_gen/base"
	idpbiz "github.com/castlexu/micro-service/services/idp/biz"
	idpgen "github.com/castlexu/micro-service/services/idp/kitex_gen/idp"
)

// IDPImpl 实现 Kitex 生成的 IDPService 接口。
type IDPImpl struct {
	loginBiz *idpbiz.LoginBiz
	tokenBiz *idpbiz.TokenBiz
	oauthBiz *idpbiz.OAuthBiz
}

// NewIDPImpl 构造 IDPImpl。
func NewIDPImpl(loginBiz *idpbiz.LoginBiz, tokenBiz *idpbiz.TokenBiz, oauthBiz *idpbiz.OAuthBiz) *IDPImpl {
	return &IDPImpl{loginBiz: loginBiz, tokenBiz: tokenBiz, oauthBiz: oauthBiz}
}

// GetGoogleAuthURL 返回 Google OAuth2 授权 URL。
func (s *IDPImpl) GetGoogleAuthURL(ctx context.Context, req *idpgen.GetGoogleAuthURLReq) (*idpgen.GetGoogleAuthURLResp, error) {
	redirectURI := ""
	if req.RedirectURI != nil {
		redirectURI = *req.RedirectURI
	}
	authURL, state, err := s.oauthBiz.GetAuthURL(ctx, redirectURI)
	if err != nil {
		return &idpgen.GetGoogleAuthURLResp{Base: errBase(err)}, nil
	}
	return &idpgen.GetGoogleAuthURLResp{
		Base:    okBase(),
		AuthURL: authURL,
		State:   state,
	}, nil
}

// LoginByGoogle 处理 Google 回调，签发本服务 JWT。
func (s *IDPImpl) LoginByGoogle(ctx context.Context, req *idpgen.LoginByGoogleReq) (*idpgen.LoginByGoogleResp, error) {
	result, err := s.loginBiz.LoginByGoogle(ctx, req.Code, req.State)
	if err != nil {
		return &idpgen.LoginByGoogleResp{Base: errBase(err)}, nil
	}
	return &idpgen.LoginByGoogleResp{
		Base:         okBase(),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
		UserID:       result.UserID,
	}, nil
}

// RefreshToken 滚动刷新 token。
func (s *IDPImpl) RefreshToken(ctx context.Context, req *idpgen.RefreshTokenReq) (*idpgen.RefreshTokenResp, error) {
	pair, err := s.tokenBiz.Refresh(ctx, req.RefreshToken)
	if err != nil {
		return &idpgen.RefreshTokenResp{Base: errBase(err)}, nil
	}
	return &idpgen.RefreshTokenResp{
		Base:         okBase(),
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.ExpiresAt,
	}, nil
}

// VerifyToken 校验 access token。
func (s *IDPImpl) VerifyToken(ctx context.Context, req *idpgen.VerifyTokenReq) (*idpgen.VerifyTokenResp, error) {
	claims, err := s.tokenBiz.Verify(ctx, req.Token)
	if err != nil {
		return &idpgen.VerifyTokenResp{Base: errBase(err)}, nil
	}
	expiresAt := int64(0)
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Unix()
	}
	tenantID := ""
	if claims.TenantID != "" {
		tenantID = claims.TenantID
	}
	return &idpgen.VerifyTokenResp{
		Base:      okBase(),
		UserID:    claims.UserID,
		TenantID:  tenantID,
		ExpiresAt: expiresAt,
	}, nil
}

// ---- helpers ----

func okBase() *idpbase.BaseResp {
	return &idpbase.BaseResp{Code: 0, Message: "ok"}
}

func errBase(err error) *idpbase.BaseResp {
	var e errno.Errno
	if errors.As(err, &e) {
		return &idpbase.BaseResp{Code: e.Code, Message: e.Message}
	}
	return &idpbase.BaseResp{Code: errno.ErrInternal.Code, Message: err.Error()}
}
