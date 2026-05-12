// Package biz — 支付宝登录主流程。
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

// AlipayLoginBiz 处理支付宝扫码登录主链路。
type AlipayLoginBiz struct {
	alipayBiz    *AlipayBiz
	tokenBiz     *TokenBiz
	identityRepo *idpmongo.IdentityRepo
	iamClient    iamclient.Client
}

// NewAlipayLoginBiz 构造 AlipayLoginBiz。
func NewAlipayLoginBiz(
	alipayBiz *AlipayBiz,
	tokenBiz *TokenBiz,
	identityRepo *idpmongo.IdentityRepo,
	iamClient iamclient.Client,
) *AlipayLoginBiz {
	return &AlipayLoginBiz{
		alipayBiz:    alipayBiz,
		tokenBiz:     tokenBiz,
		identityRepo: identityRepo,
		iamClient:    iamClient,
	}
}

// LoginByAlipay 处理支付宝回调：
//  1. 验证 state、换取支付宝 access_token 并拉取用户信息
//  2. 调 iam UpsertUserByProvider（以 alipay_user_id@alipay 作为 email 唯一键）
//  3. 更新/创建 idp identity 映射
//  4. 签发本服务 JWT
func (b *AlipayLoginBiz) LoginByAlipay(ctx context.Context, authCode, state string) (*LoginResult, error) {
	if authCode == "" {
		return nil, errno.ErrInvalidParam.WithMessage("idp: auth_code is required")
	}
	if state == "" {
		return nil, errno.ErrInvalidParam.WithMessage("idp: state is required")
	}

	// 1. 验 state + 换 token + 拉用户信息
	info, err := b.alipayBiz.ExchangeCode(ctx, authCode, state)
	if err != nil {
		return nil, err
	}

	// 支付宝不返回 email；用 userID@alipay 作为内部唯一 email 标识
	internalEmail := info.UserID + "@alipay.user"

	// 2. 调 iam 创建/更新用户
	iamResp, err := b.iamClient.UpsertUserByProvider(ctx, &iamgen.UpsertUserByProviderReq{
		Base: &iambase.BaseReq{},
		Profile: &iamgen.ProviderProfile{
			Provider:    "alipay",
			ProviderSub: info.UserID,
			Email:       internalEmail,
			Name:        strPtrOrNil(info.NickName),
			AvatarURL:   strPtrOrNil(info.Avatar),
		},
	})
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("idp: call iam upsert: %v", err)
	}
	if iamResp.GetBase() != nil && iamResp.Base.Code != 0 {
		return nil, errno.ErrInternal.WithMessagef("idp: iam upsert error: %s", iamResp.Base.Message)
	}
	// 检查用户状态
	if iamResp.Status == 2 || iamResp.Status == 3 {
		return nil, errno.ErrAccountLocked.WithMessage("idp: account is disabled or banned")
	}

	// 3. 更新 identity 映射
	iamUserID, err := primitive.ObjectIDFromHex(iamResp.UserID)
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("idp: invalid iam user_id: %v", err)
	}
	if _, _, err := b.identityRepo.Upsert(ctx, "alipay", info.UserID, iamUserID, internalEmail); err != nil {
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
