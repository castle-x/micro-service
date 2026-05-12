// Package handler 只做参数校验 + 调 RPC + 组装响应，禁止直接访问数据库。
package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/logger"
	edgebase "github.com/castlexu/micro-service/services/edge-api/kitex_gen/base"
	idpgen "github.com/castlexu/micro-service/services/edge-api/kitex_gen/idp"
	idpclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/idp/idpservice"
)

// AuthHandler 处理 /api/v1/auth/* 路由。
type AuthHandler struct {
	idpClient   idpclient.Client
	frontendURL string
}

// NewAuthHandler 构造 AuthHandler。
func NewAuthHandler(idpClient idpclient.Client, frontendURL string) *AuthHandler {
	return &AuthHandler{idpClient: idpClient, frontendURL: frontendURL}
}

// ---- 请求/响应结构 ----

type googleAuthURLResp struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginByPasswordReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	UserID       string `json:"user_id"`
}

type refreshTokenReq struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutReq struct {
	RefreshToken string `json:"refresh_token"`
}

type apiResp struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ---- Handlers ----

// GetGoogleAuthURL GET /api/v1/auth/google/url
// 返回 Google OAuth2 授权 URL 和防 CSRF state。
func (h *AuthHandler) GetGoogleAuthURL(c context.Context, ctx *app.RequestContext) {
	idpResp, err := h.idpClient.GetGoogleAuthURL(c, &idpgen.GetGoogleAuthURLReq{
		Base: &edgebase.BaseReq{},
	})
	if err != nil {
		logger.Ctx(c).Error("idp.GetGoogleAuthURL failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if idpResp.Base != nil && idpResp.Base.Code != 0 {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: idpResp.Base.Code, Message: idpResp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "ok",
		Data:    googleAuthURLResp{AuthURL: idpResp.AuthURL, State: idpResp.State},
	})
}

// GoogleCallback GET /api/v1/auth/google/callback
// 接收 Google OAuth2 回调，调 idp 完成登录后重定向到前端。
// 成功：重定向到 {FRONTEND_URL}/auth/callback?access_token=...
// 失败：重定向到 {FRONTEND_URL}/login?error=...
func (h *AuthHandler) GoogleCallback(c context.Context, ctx *app.RequestContext) {
	code := string(ctx.Query("code"))
	state := string(ctx.Query("state"))
	if code == "" || state == "" {
		redirectURL := fmt.Sprintf("%s/login?error=%s", h.frontendURL, url.QueryEscape("code and state are required"))
		ctx.Redirect(http.StatusFound, []byte(redirectURL))
		return
	}

	idpResp, err := h.idpClient.LoginByGoogle(c, &idpgen.LoginByGoogleReq{
		Base:  &edgebase.BaseReq{},
		Code:  code,
		State: state,
	})
	if err != nil {
		logger.Ctx(c).Error("idp.LoginByGoogle failed", zap.Error(err))
		redirectURL := fmt.Sprintf("%s/login?error=%s", h.frontendURL, url.QueryEscape(err.Error()))
		ctx.Redirect(http.StatusFound, []byte(redirectURL))
		return
	}
	if idpResp.Base != nil && idpResp.Base.Code != 0 {
		redirectURL := fmt.Sprintf("%s/login?error=%s", h.frontendURL, url.QueryEscape(idpResp.Base.Message))
		ctx.Redirect(http.StatusFound, []byte(redirectURL))
		return
	}

	redirectURL := fmt.Sprintf(
		"%s/auth/callback?access_token=%s&refresh_token=%s&user_id=%s&expires_at=%d",
		h.frontendURL,
		url.QueryEscape(idpResp.AccessToken),
		url.QueryEscape(idpResp.RefreshToken),
		url.QueryEscape(idpResp.UserID),
		idpResp.ExpiresAt,
	)
	ctx.Redirect(http.StatusFound, []byte(redirectURL))
}

// RefreshToken POST /api/v1/auth/token/refresh
func (h *AuthHandler) RefreshToken(c context.Context, ctx *app.RequestContext) {
	var req refreshTokenReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	if req.RefreshToken == "" {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "refresh_token is required"})
		return
	}

	idpResp, err := h.idpClient.RefreshToken(c, &idpgen.RefreshTokenReq{
		Base:         &edgebase.BaseReq{},
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		logger.Ctx(c).Error("idp.RefreshToken failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if idpResp.Base != nil && idpResp.Base.Code != 0 {
		httpCode := bizCodeToHTTP(idpResp.Base.Code)
		ctx.JSON(httpCode, apiResp{Code: idpResp.Base.Code, Message: idpResp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "ok",
		Data: authTokenResp{
			AccessToken:  idpResp.AccessToken,
			RefreshToken: idpResp.RefreshToken,
			ExpiresAt:    idpResp.ExpiresAt,
		},
	})
}

// Logout POST /api/v1/auth/logout
// 简化实现：验证 access token 后返回 200，前端清除 localStorage。
// TODO Phase 04: 通过 idp 服务撤销 refresh token JTI。
func (h *AuthHandler) Logout(c context.Context, ctx *app.RequestContext) {
	var req logoutReq
	_ = ctx.BindJSON(&req) // best-effort，body 解析失败不影响登出

	// 验证 access token（best-effort，错误不阻断登出流程）
	authHeader := string(ctx.GetHeader("Authorization"))
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		_, _ = h.idpClient.VerifyToken(c, &idpgen.VerifyTokenReq{
			Base:  &edgebase.BaseReq{},
			Token: token,
		})
	}

	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok"})
}

// GetAlipayAuthURL GET /api/v1/auth/alipay/url
func (h *AuthHandler) GetAlipayAuthURL(c context.Context, ctx *app.RequestContext) {
	idpResp, err := h.idpClient.GetAlipayAuthURL(c, &idpgen.GetAlipayAuthURLReq{
		Base: &edgebase.BaseReq{},
	})
	if err != nil {
		logger.Ctx(c).Error("idp.GetAlipayAuthURL failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if idpResp.Base != nil && idpResp.Base.Code != 0 {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: idpResp.Base.Code, Message: idpResp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "ok",
		Data:    map[string]string{"auth_url": idpResp.AuthURL, "state": idpResp.State},
	})
}

// AlipayCallback GET /api/v1/auth/alipay/callback
// 支付宝回调携带 auth_code 和 state，调 idp 完成登录后重定向到前端。
func (h *AuthHandler) AlipayCallback(c context.Context, ctx *app.RequestContext) {
	authCode := string(ctx.Query("auth_code"))
	state := string(ctx.Query("state"))
	if authCode == "" || state == "" {
		redirectURL := fmt.Sprintf("%s/login?error=%s", h.frontendURL, url.QueryEscape("auth_code and state are required"))
		ctx.Redirect(http.StatusFound, []byte(redirectURL))
		return
	}

	idpResp, err := h.idpClient.LoginByAlipay(c, &idpgen.LoginByAlipayReq{
		Base:     &edgebase.BaseReq{},
		AuthCode: authCode,
		State:    state,
	})
	if err != nil {
		logger.Ctx(c).Error("idp.LoginByAlipay failed", zap.Error(err))
		redirectURL := fmt.Sprintf("%s/login?error=%s", h.frontendURL, url.QueryEscape(err.Error()))
		ctx.Redirect(http.StatusFound, []byte(redirectURL))
		return
	}
	if idpResp.Base != nil && idpResp.Base.Code != 0 {
		redirectURL := fmt.Sprintf("%s/login?error=%s", h.frontendURL, url.QueryEscape(idpResp.Base.Message))
		ctx.Redirect(http.StatusFound, []byte(redirectURL))
		return
	}

	redirectURL := fmt.Sprintf(
		"%s/auth/callback?access_token=%s&refresh_token=%s&user_id=%s&expires_at=%d",
		h.frontendURL,
		url.QueryEscape(idpResp.AccessToken),
		url.QueryEscape(idpResp.RefreshToken),
		url.QueryEscape(idpResp.UserID),
		idpResp.ExpiresAt,
	)
	ctx.Redirect(http.StatusFound, []byte(redirectURL))
}

// Register POST /api/v1/auth/register
func (h *AuthHandler) Register(c context.Context, ctx *app.RequestContext) {
	var req registerReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	var name *string
	if req.Name != "" {
		name = &req.Name
	}
	idpResp, err := h.idpClient.Register(c, &idpgen.RegisterReq{
		Base:     &edgebase.BaseReq{},
		Email:    req.Email,
		Password: req.Password,
		Name:     name,
	})
	if err != nil {
		logger.Ctx(c).Error("idp.Register failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if idpResp.Base != nil && idpResp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(idpResp.Base.Code), apiResp{Code: idpResp.Base.Code, Message: idpResp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "ok",
		Data: authTokenResp{
			AccessToken:  idpResp.AccessToken,
			RefreshToken: idpResp.RefreshToken,
			ExpiresAt:    idpResp.ExpiresAt,
			UserID:       idpResp.UserID,
		},
	})
}

// LoginByPassword POST /api/v1/auth/login
func (h *AuthHandler) LoginByPassword(c context.Context, ctx *app.RequestContext) {
	var req loginByPasswordReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	idpResp, err := h.idpClient.LoginByPassword(c, &idpgen.LoginByPasswordReq{
		Base:     &edgebase.BaseReq{},
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		logger.Ctx(c).Error("idp.LoginByPassword failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if idpResp.Base != nil && idpResp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(idpResp.Base.Code), apiResp{Code: idpResp.Base.Code, Message: idpResp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "ok",
		Data: authTokenResp{
			AccessToken:  idpResp.AccessToken,
			RefreshToken: idpResp.RefreshToken,
			ExpiresAt:    idpResp.ExpiresAt,
			UserID:       idpResp.UserID,
		},
	})
}

// bizCodeToHTTP 将业务错误码转换为 HTTP 状态码。
func bizCodeToHTTP(code int32) int {
	var e errno.Errno
	e.Code = code
	switch {
	case errors.Is(e, errno.ErrInvalidParam):
		return http.StatusBadRequest
	case errors.Is(e, errno.ErrUnauthorized), errors.Is(e, errno.ErrTokenInvalid), errors.Is(e, errno.ErrTokenExpired):
		return http.StatusUnauthorized
	case errors.Is(e, errno.ErrForbidden), errors.Is(e, errno.ErrPermissionDenied):
		return http.StatusForbidden
	case errors.Is(e, errno.ErrNotFound), errors.Is(e, errno.ErrUserNotFound):
		return http.StatusNotFound
	case errors.Is(e, errno.ErrRateLimit):
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}
