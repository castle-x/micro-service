// Package biz — 支付宝 OAuth2 登录（RSA2 签名）。
//
// 支付宝与 Google OAuth2 的主要差异：
//   - 不使用 client_secret，而是 RSA2 私钥对请求参数签名
//   - 没有 id_token，需要两步：先换 access_token，再调 alipay.user.info.share 拉用户信息
//   - auth_code 有效期极短（约 3 分钟），需立即换 token
//   - 用户唯一 ID 是 user_id（2088 开头），等价于微信 openid
package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/castlexu/micro-service/pkg/errno"
	idpmongo "github.com/castlexu/micro-service/services/idp/dal/mongo"
)

const (
	alipayGateway   = "https://openapi.alipaydev.com/gateway.do" // 沙箱网关，生产改为 openapi.alipay.com
	alipayAuthURL   = "https://openauth.alipaydev.com/oauth2/publicAppAuthorize.htm"
	alipayScope     = "auth_user"
)

// AlipayBiz 处理支付宝 OAuth2 扫码登录流程。
type AlipayBiz struct {
	appID       string
	privateKey  string // PKCS8 格式 RSA2 私钥（PEM，不含头尾）
	alipayPubKey string // 支付宝公钥（用于验签，暂未实现，可按需补充）
	redirectURL string
	stateRepo   *idpmongo.OAuthStateRepo
	gateway     string
}

// AlipayConfig 支付宝登录配置。
type AlipayConfig struct {
	AppID        string
	PrivateKey   string // 应用私钥，PKCS8，不含 PEM 头尾
	AlipayPubKey string // 支付宝公钥
	RedirectURL  string
	Sandbox      bool // true = 沙箱环境
}

// NewAlipayBiz 构造 AlipayBiz。
func NewAlipayBiz(cfg AlipayConfig, stateRepo *idpmongo.OAuthStateRepo) *AlipayBiz {
	gateway := "https://openapi.alipay.com/gateway.do"
	authBase := "https://openauth.alipay.com/oauth2/publicAppAuthorize.htm"
	if cfg.Sandbox {
		gateway = "https://openapi.alipaydev.com/gateway.do"
		authBase = "https://openauth.alipaydev.com/oauth2/publicAppAuthorize.htm"
	}
	_ = authBase // authBase 拼在 GetAuthURL 里
	return &AlipayBiz{
		appID:        cfg.AppID,
		privateKey:   cfg.PrivateKey,
		alipayPubKey: cfg.AlipayPubKey,
		redirectURL:  cfg.RedirectURL,
		stateRepo:    stateRepo,
		gateway:      gateway,
	}
}

// GetAuthURL 生成支付宝扫码授权 URL 并保存 state。
func (b *AlipayBiz) GetAuthURL(ctx context.Context, overrideRedirect string) (authURL, state string, err error) {
	state, err = generateState()
	if err != nil {
		return "", "", errno.ErrInternal.WithMessagef("alipay: generate state: %v", err)
	}
	redirect := b.redirectURL
	if overrideRedirect != "" {
		redirect = overrideRedirect
	}
	if err := b.stateRepo.Save(ctx, state, redirect); err != nil {
		return "", "", err
	}

	base := "https://openauth.alipaydev.com/oauth2/publicAppAuthorize.htm"
	if !strings.Contains(b.gateway, "dev") {
		base = "https://openauth.alipay.com/oauth2/publicAppAuthorize.htm"
	}

	params := url.Values{}
	params.Set("app_id", b.appID)
	params.Set("scope", alipayScope)
	params.Set("redirect_uri", redirect)
	params.Set("state", state)
	authURL = fmt.Sprintf("%s?%s", base, params.Encode())
	return authURL, state, nil
}

// AlipayUserInfo 支付宝用户信息。
type AlipayUserInfo struct {
	UserID    string // 支付宝 user_id（2088 开头）
	NickName  string
	Avatar    string
	// 支付宝不返回 email，以 user_id@alipay 作为内部 email 唯一标识
}

// ExchangeCode 验证 state 并用 auth_code 换取用户信息。
func (b *AlipayBiz) ExchangeCode(ctx context.Context, authCode, state string) (*AlipayUserInfo, error) {
	if authCode == "" {
		return nil, errno.ErrInvalidParam.WithMessage("alipay: auth_code is required")
	}

	// 1. 消费 state
	if _, err := b.stateRepo.ConsumeAndDelete(ctx, state); err != nil {
		return nil, err
	}

	// 2. auth_code → access_token
	tokenResp, err := b.systemOauthToken(ctx, authCode)
	if err != nil {
		return nil, err
	}

	// 3. access_token → 用户信息
	userInfo, err := b.userInfoShare(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, err
	}
	return userInfo, nil
}

// ---- 支付宝 API 调用 ----

type alipayTokenResponse struct {
	AlipaySystemOauthTokenResponse struct {
		UserID       string `json:"user_id"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    string `json:"expires_in"`
	} `json:"alipay_system_oauth_token_response"`
	ErrorResponse *alipayErrorResponse `json:"error_response,omitempty"`
}

type alipayUserInfoResponse struct {
	AlipayUserInfoShareResponse struct {
		UserID   string `json:"user_id"`
		NickName string `json:"nick_name"`
		Avatar   string `json:"avatar"`
	} `json:"alipay_user_info_share_response"`
	ErrorResponse *alipayErrorResponse `json:"error_response,omitempty"`
}

type alipayErrorResponse struct {
	Code    string `json:"code"`
	Msg     string `json:"msg"`
	SubCode string `json:"sub_code"`
	SubMsg  string `json:"sub_msg"`
}

func (b *AlipayBiz) systemOauthToken(ctx context.Context, authCode string) (*struct{ AccessToken string }, error) {
	bizContent := fmt.Sprintf(`{"grant_type":"authorization_code","code":"%s"}`, authCode)
	params, err := b.buildParams("alipay.system.oauth.token", bizContent)
	if err != nil {
		return nil, err
	}

	body, err := b.doPost(ctx, params)
	if err != nil {
		return nil, err
	}

	var resp alipayTokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errno.ErrInternal.WithMessagef("alipay: parse token response: %v", err)
	}
	if resp.ErrorResponse != nil {
		return nil, errno.ErrInvalidCredentials.WithMessagef("alipay: token error %s: %s", resp.ErrorResponse.SubCode, resp.ErrorResponse.SubMsg)
	}
	t := resp.AlipaySystemOauthTokenResponse
	if t.AccessToken == "" {
		return nil, errno.ErrInvalidCredentials.WithMessage("alipay: empty access_token")
	}
	return &struct{ AccessToken string }{AccessToken: t.AccessToken}, nil
}

func (b *AlipayBiz) userInfoShare(ctx context.Context, accessToken string) (*AlipayUserInfo, error) {
	bizContent := fmt.Sprintf(`{"auth_token":"%s"}`, accessToken)
	params, err := b.buildParams("alipay.user.info.share", bizContent)
	if err != nil {
		return nil, err
	}

	body, err := b.doPost(ctx, params)
	if err != nil {
		return nil, err
	}

	var resp alipayUserInfoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, errno.ErrInternal.WithMessagef("alipay: parse userinfo response: %v", err)
	}
	if resp.ErrorResponse != nil {
		return nil, errno.ErrInternal.WithMessagef("alipay: userinfo error %s: %s", resp.ErrorResponse.SubCode, resp.ErrorResponse.SubMsg)
	}
	u := resp.AlipayUserInfoShareResponse
	if u.UserID == "" {
		return nil, errno.ErrInternal.WithMessage("alipay: empty user_id in userinfo")
	}
	return &AlipayUserInfo{
		UserID:   u.UserID,
		NickName: u.NickName,
		Avatar:   u.Avatar,
	}, nil
}

// buildParams 构造支付宝请求参数并添加 RSA2 签名。
func (b *AlipayBiz) buildParams(method, bizContent string) (url.Values, error) {
	params := map[string]string{
		"app_id":      b.appID,
		"method":      method,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": bizContent,
	}

	// 按 key 排序拼接待签名字符串
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	signContent := strings.Join(parts, "&")

	sig, err := rsaSign(signContent, b.privateKey)
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("alipay: sign: %v", err)
	}
	params["sign"] = sig

	values := make(url.Values)
	for k, v := range params {
		values.Set(k, v)
	}
	return values, nil
}

// doPost 发起 HTTP POST 到支付宝网关。
func (b *AlipayBiz) doPost(ctx context.Context, params url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.gateway, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("alipay: build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errno.ErrServiceUnavailable.WithMessagef("alipay: http post: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("alipay: read response: %v", err)
	}
	return body, nil
}
