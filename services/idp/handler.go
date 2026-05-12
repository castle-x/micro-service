package main

import (
	"context"
	"errors"

	"github.com/castlexu/micro-service/pkg/errno"
	idpbiz "github.com/castlexu/micro-service/services/idp/biz"
	idpbase "github.com/castlexu/micro-service/services/idp/kitex_gen/base"
	idpgen "github.com/castlexu/micro-service/services/idp/kitex_gen/idp"
)

// IDPImpl 实现 Kitex 生成的 IDPService 接口。
type IDPImpl struct {
	loginBiz         *idpbiz.LoginBiz
	alipayLoginBiz   *idpbiz.AlipayLoginBiz
	passwordLoginBiz *idpbiz.PasswordLoginBiz
	tokenBiz         *idpbiz.TokenBiz
	oauthBiz         *idpbiz.OAuthBiz
	alipayBiz        *idpbiz.AlipayBiz
}

// NewIDPImpl 构造 IDPImpl。
func NewIDPImpl(
	loginBiz *idpbiz.LoginBiz,
	alipayLoginBiz *idpbiz.AlipayLoginBiz,
	passwordLoginBiz *idpbiz.PasswordLoginBiz,
	tokenBiz *idpbiz.TokenBiz,
	oauthBiz *idpbiz.OAuthBiz,
	alipayBiz *idpbiz.AlipayBiz,
) *IDPImpl {
	return &IDPImpl{
		loginBiz:         loginBiz,
		alipayLoginBiz:   alipayLoginBiz,
		passwordLoginBiz: passwordLoginBiz,
		tokenBiz:         tokenBiz,
		oauthBiz:         oauthBiz,
		alipayBiz:        alipayBiz,
	}
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

// GetAlipayAuthURL 返回支付宝扫码授权 URL。
func (s *IDPImpl) GetAlipayAuthURL(ctx context.Context, req *idpgen.GetAlipayAuthURLReq) (*idpgen.GetAlipayAuthURLResp, error) {
	redirectURI := ""
	if req.RedirectURI != nil {
		redirectURI = *req.RedirectURI
	}
	authURL, state, err := s.alipayBiz.GetAuthURL(ctx, redirectURI)
	if err != nil {
		return &idpgen.GetAlipayAuthURLResp{Base: errBase(err)}, nil
	}
	return &idpgen.GetAlipayAuthURLResp{
		Base:    okBase(),
		AuthURL: authURL,
		State:   state,
	}, nil
}

// LoginByAlipay 处理支付宝回调，签发本服务 JWT。
func (s *IDPImpl) LoginByAlipay(ctx context.Context, req *idpgen.LoginByAlipayReq) (*idpgen.LoginByAlipayResp, error) {
	result, err := s.alipayLoginBiz.LoginByAlipay(ctx, req.AuthCode, req.State)
	if err != nil {
		return &idpgen.LoginByAlipayResp{Base: errBase(err)}, nil
	}
	return &idpgen.LoginByAlipayResp{
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

// Register 注册新用户并签发 token。
func (s *IDPImpl) Register(ctx context.Context, req *idpgen.RegisterReq) (*idpgen.RegisterResp, error) {
	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	result, err := s.passwordLoginBiz.Register(ctx, req.Email, req.Password, name)
	if err != nil {
		return &idpgen.RegisterResp{Base: errBase(err)}, nil
	}
	return &idpgen.RegisterResp{
		Base:         okBase(),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
		UserID:       result.UserID,
	}, nil
}

// LoginByPassword 账号密码登录。
func (s *IDPImpl) LoginByPassword(ctx context.Context, req *idpgen.LoginByPasswordReq) (*idpgen.LoginByPasswordResp, error) {
	result, err := s.passwordLoginBiz.LoginByPassword(ctx, req.Email, req.Password)
	if err != nil {
		return &idpgen.LoginByPasswordResp{Base: errBase(err)}, nil
	}
	return &idpgen.LoginByPasswordResp{
		Base:         okBase(),
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    result.ExpiresAt,
		UserID:       result.UserID,
	}, nil
}

// RevokeUserTokens 撤销指定用户的所有 refresh token（角色变更后调用）。
func (s *IDPImpl) RevokeUserTokens(ctx context.Context, req *idpgen.RevokeUserTokensReq) (*idpgen.RevokeUserTokensResp, error) {
	if err := s.tokenBiz.RevokeUserTokens(ctx, req.UserID); err != nil {
		return &idpgen.RevokeUserTokensResp{Base: errBase(err)}, nil
	}
	return &idpgen.RevokeUserTokensResp{Base: okBase()}, nil
}

// BanUser 封禁用户（写封禁标记 + 撤销所有 refresh token）。
func (s *IDPImpl) BanUser(ctx context.Context, req *idpgen.BanUserReq) (*idpgen.BanUserResp, error) {
	if err := s.tokenBiz.BanUser(ctx, req.UserID); err != nil {
		return &idpgen.BanUserResp{Base: errBase(err)}, nil
	}
	return &idpgen.BanUserResp{Base: okBase()}, nil
}

// UnbanUser 解除封禁。
func (s *IDPImpl) UnbanUser(ctx context.Context, req *idpgen.UnbanUserReq) (*idpgen.UnbanUserResp, error) {
	if err := s.tokenBiz.UnbanUser(ctx, req.UserID); err != nil {
		return &idpgen.UnbanUserResp{Base: errBase(err)}, nil
	}
	return &idpgen.UnbanUserResp{Base: okBase()}, nil
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
