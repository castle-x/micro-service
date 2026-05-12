// Package biz — 账号密码注册与登录流程。
package biz

import (
	"context"
	"regexp"
	"unicode"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"

	"github.com/castlexu/micro-service/pkg/errno"
	idpmongo "github.com/castlexu/micro-service/services/idp/dal/mongo"
	iambase "github.com/castlexu/micro-service/services/idp/kitex_gen/base"
	iamgen "github.com/castlexu/micro-service/services/idp/kitex_gen/iam"
	iamclient "github.com/castlexu/micro-service/services/idp/kitex_gen/iam/iamservice"
)

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// PasswordLoginBiz 处理账号密码注册与登录。
type PasswordLoginBiz struct {
	tokenBiz  *TokenBiz
	credRepo  *idpmongo.PasswordCredRepo
	iamClient iamclient.Client
}

// NewPasswordLoginBiz 构造 PasswordLoginBiz。
func NewPasswordLoginBiz(
	tokenBiz *TokenBiz,
	credRepo *idpmongo.PasswordCredRepo,
	iamClient iamclient.Client,
) *PasswordLoginBiz {
	return &PasswordLoginBiz{
		tokenBiz:  tokenBiz,
		credRepo:  credRepo,
		iamClient: iamClient,
	}
}

// Register 注册新用户并签发 token：
//  1. 参数校验（email 格式、密码复杂度）
//  2. 调 iam UpsertUserByProvider 创建用户（provider="password", sub=email）
//  3. 写入 password_credentials（email 重复时返回错误）
//  4. 签发 JWT
func (b *PasswordLoginBiz) Register(ctx context.Context, email, password, name string) (*LoginResult, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	// 1. 调 iam 创建用户
	iamResp, err := b.iamClient.UpsertUserByProvider(ctx, &iamgen.UpsertUserByProviderReq{
		Base: &iambase.BaseReq{},
		Profile: &iamgen.ProviderProfile{
			Provider:    "password",
			ProviderSub: email,
			Email:       email,
			Name:        strPtrOrNil(name),
		},
	})
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("idp: call iam upsert: %v", err)
	}
	if iamResp.GetBase() != nil && iamResp.Base.Code != 0 {
		return nil, errno.ErrInternal.WithMessagef("idp: iam upsert error: %s", iamResp.Base.Message)
	}

	iamUserID, err := primitive.ObjectIDFromHex(iamResp.UserID)
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("idp: invalid iam user_id: %v", err)
	}

	// 2. hash 密码并写库
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("idp: bcrypt: %v", err)
	}
	if err := b.credRepo.Insert(ctx, iamUserID, email, string(hash)); err != nil {
		return nil, err // errno.ErrDuplicateKey 或 ErrInternal，直接透传
	}

	// 3. 签发 JWT（携带 role）
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

// LoginByPassword 账号密码登录：
//  1. 按 email 查凭据
//  2. bcrypt 校验密码
//  3. 从 IAM 查 role
//  4. 签发 JWT
func (b *PasswordLoginBiz) LoginByPassword(ctx context.Context, email, password string) (*LoginResult, error) {
	if email == "" || password == "" {
		return nil, errno.ErrInvalidParam.WithMessage("idp: email and password are required")
	}

	cred, err := b.credRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, errno.ErrInvalidCredentials.WithMessage("idp: invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(password)); err != nil {
		return nil, errno.ErrInvalidCredentials.WithMessage("idp: invalid email or password")
	}

	// 查 IAM 获取用户 role
	iamResp, err := b.iamClient.GetUser(ctx, &iamgen.GetUserReq{
		Base:   &iambase.BaseReq{},
		UserID: cred.UserID.Hex(),
	})
	if err != nil {
		return nil, errno.ErrInternal.WithMessagef("idp: get user role: %v", err)
	}
	role := "user"
	if iamResp.GetBase() == nil || iamResp.Base.Code == 0 {
		role = iamResp.Role
		// 检查状态：disabled(2) / banned(3) 拒绝登录
		if iamResp.Status == 2 || iamResp.Status == 3 {
			return nil, errno.ErrAccountLocked.WithMessage("idp: account is disabled or banned")
		}
	}

	pair, err := b.tokenBiz.Issue(ctx, cred.UserID.Hex(), role)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.ExpiresAt,
		UserID:       cred.UserID.Hex(),
	}, nil
}

// validateEmail 校验 email 格式。
func validateEmail(email string) error {
	if !emailRe.MatchString(email) {
		return errno.ErrInvalidParam.WithMessage("idp: invalid email format")
	}
	return nil
}

// validatePassword 校验密码复杂度：
//   - 长度 ≥ 8
//   - 至少一个大写字母
//   - 至少一个小写字母
//   - 至少一个数字
//   - 至少一个特殊字符
func validatePassword(password string) error {
	if len(password) < 8 {
		return errno.ErrInvalidParam.WithMessage("idp: password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}
	if !hasUpper {
		return errno.ErrInvalidParam.WithMessage("idp: password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errno.ErrInvalidParam.WithMessage("idp: password must contain at least one lowercase letter")
	}
	if !hasDigit {
		return errno.ErrInvalidParam.WithMessage("idp: password must contain at least one digit")
	}
	if !hasSpecial {
		return errno.ErrInvalidParam.WithMessage("idp: password must contain at least one special character")
	}
	return nil
}
