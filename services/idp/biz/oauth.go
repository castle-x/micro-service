// Package biz 是 idp 核心业务逻辑层 — OAuth2 部分。
package biz

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/castlexu/micro-service/pkg/errno"
	idpmongo "github.com/castlexu/micro-service/services/idp/dal/mongo"
)

// OAuthBiz 处理 Google OAuth2 流程。
type OAuthBiz struct {
	cfg       *oauth2.Config
	stateRepo *idpmongo.OAuthStateRepo
}

// NewOAuthBiz 构造 OAuthBiz。
func NewOAuthBiz(clientID, clientSecret, redirectURL string, stateRepo *idpmongo.OAuthStateRepo) *OAuthBiz {
	cfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
	return &OAuthBiz{cfg: cfg, stateRepo: stateRepo}
}

// GetAuthURL 生成 Google OAuth2 授权 URL 并存储 state。
func (b *OAuthBiz) GetAuthURL(ctx context.Context, overrideRedirect string) (authURL, state string, err error) {
	state, err = generateState()
	if err != nil {
		return "", "", errno.ErrInternal.WithMessagef("idp: generate state: %v", err)
	}
	cfg := b.cfg
	if overrideRedirect != "" {
		// 克隆 config，避免并发修改
		copied := *b.cfg
		copied.RedirectURL = overrideRedirect
		cfg = &copied
	}
	if err := b.stateRepo.Save(ctx, state, cfg.RedirectURL); err != nil {
		return "", "", err
	}
	authURL = cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return authURL, state, nil
}

// ExchangeCode 用授权码换取 Google token 并验证 state。
// 返回 GoogleUserInfo。
func (b *OAuthBiz) ExchangeCode(ctx context.Context, code, state string) (*GoogleUserInfo, error) {
	// 消费 state（防 CSRF，一次性）
	stateDoc, err := b.stateRepo.ConsumeAndDelete(ctx, state)
	if err != nil {
		return nil, err
	}

	cfg := b.cfg
	if stateDoc.RedirectURI != "" && stateDoc.RedirectURI != b.cfg.RedirectURL {
		copied := *b.cfg
		copied.RedirectURL = stateDoc.RedirectURI
		cfg = &copied
	}

	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, errno.ErrInvalidCredentials.WithMessagef("idp: exchange code: %v", err)
	}

	return parseGoogleIDToken(token)
}

// GoogleUserInfo 是从 Google ID token 解析出的用户信息。
type GoogleUserInfo struct {
	Sub       string // Google subject
	Email     string
	Name      string
	AvatarURL string
}

// parseGoogleIDToken 从 oauth2.Token 的 id_token 字段解析用户信息。
// Google id_token 是 JWT，直接 base64 解码 payload 即可（无需验签，已由 Exchange 完成）。
func parseGoogleIDToken(token *oauth2.Token) (*GoogleUserInfo, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errno.ErrInvalidCredentials.WithMessage("idp: missing id_token in google response")
	}
	return decodeIDToken(rawIDToken)
}

// generateState 生成 32 字节随机 state，base64url 编码。
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
