// Package biz — Google 登录主流程。
package biz

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/castlexu/micro-service/pkg/errno"
	idpmongo "github.com/castlexu/micro-service/services/idp/dal/mongo"
	iambase "github.com/castlexu/micro-service/services/idp/kitex_gen/base"
	iamgen "github.com/castlexu/micro-service/services/idp/kitex_gen/iam"
	iamclient "github.com/castlexu/micro-service/services/idp/kitex_gen/iam/iamservice"
)

// LoginBiz 处理 Google OAuth2 登录主链路。
type LoginBiz struct {
	oauthBiz     *OAuthBiz
	tokenBiz     *TokenBiz
	identityRepo *idpmongo.IdentityRepo
	iamClient    iamclient.Client
}

// NewLoginBiz 构造 LoginBiz。
func NewLoginBiz(
	oauthBiz *OAuthBiz,
	tokenBiz *TokenBiz,
	identityRepo *idpmongo.IdentityRepo,
	iamClient iamclient.Client,
) *LoginBiz {
	return &LoginBiz{
		oauthBiz:     oauthBiz,
		tokenBiz:     tokenBiz,
		identityRepo: identityRepo,
		iamClient:    iamClient,
	}
}

// LoginResult 是登录成功后的返回结果。
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
	UserID       string
}

// LoginByGoogle 处理 Google OAuth2 回调：
//  1. 验证 state、换取 Google token
//  2. 解析 id_token 拿到用户信息
//  3. 调 iam 服务 UpsertUserByProvider
//  4. 更新/创建 idp identity 映射
//  5. 签发本服务 JWT
func (b *LoginBiz) LoginByGoogle(ctx context.Context, code, state string) (*LoginResult, error) {
	if code == "" {
		return nil, errno.ErrInvalidParam.WithMessage("idp: code is required")
	}
	if state == "" {
		return nil, errno.ErrInvalidParam.WithMessage("idp: state is required")
	}

	// 1. 换取 Google token + 解析 id_token
	info, err := b.oauthBiz.ExchangeCode(ctx, code, state)
	if err != nil {
		return nil, err
	}

	// 2. 调 iam 创建/更新用户
	iamResp, err := b.iamClient.UpsertUserByProvider(ctx, &iamgen.UpsertUserByProviderReq{
		Base: &iambase.BaseReq{},
		Profile: &iamgen.ProviderProfile{
			Provider:    "google",
			ProviderSub: info.Sub,
			Email:       info.Email,
			Name:        strPtrOrNil(info.Name),
			AvatarURL:   strPtrOrNil(info.AvatarURL),
		},
	})
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("idp: call iam upsert: %v", err)
	}
	if iamResp.GetBase() != nil && iamResp.Base.Code != 0 {
		return nil, errno.ErrInternal.WithMessagef("idp: iam upsert error: %s", iamResp.Base.Message)
	}
	// 检查用户状态：disabled(2) 和 banned(3) 拒绝登录
	if iamResp.Status == 2 || iamResp.Status == 3 {
		return nil, errno.ErrAccountLocked.WithMessage("idp: account is disabled or banned")
	}
	// 检查用户状态：disabled(2) 和 banned(3) 拒绝登录
	if iamResp.Status == 2 || iamResp.Status == 3 {
		return nil, errno.ErrAccountLocked.WithMessage("idp: account is disabled or banned")
	}

	// 3. 更新 idp identity 映射
	iamUserID, err := primitive.ObjectIDFromHex(iamResp.UserID)
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("idp: invalid iam user_id: %v", err)
	}
	if _, _, err := b.identityRepo.Upsert(ctx, "google", info.Sub, iamUserID, info.Email); err != nil {
		return nil, err
	}

	// 4. 签发 JWT（携带 role）
	pair, err := b.tokenBiz.Issue(ctx, iamResp.UserID, iamResp.Role)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.ExpiresAt,
		UserID:       iamResp.UserID,
	}, nil
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
