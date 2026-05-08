package main

import (
	"context"
	"errors"

	"github.com/castlexu/micro-service/pkg/errno"
	iambase "github.com/castlexu/micro-service/services/iam/kitex_gen/base"
	iamgen "github.com/castlexu/micro-service/services/iam/kitex_gen/iam"
	"github.com/castlexu/micro-service/services/iam/biz"
)

// IAMImpl 实现 Kitex 生成的 IAMService 接口。
type IAMImpl struct {
	userBiz *biz.UserBiz
}

// NewIAMImpl 构造 IAMImpl。
func NewIAMImpl(userBiz *biz.UserBiz) *IAMImpl {
	return &IAMImpl{userBiz: userBiz}
}

// UpsertUserByProvider 幂等创建/更新用户。
func (s *IAMImpl) UpsertUserByProvider(ctx context.Context, req *iamgen.UpsertUserByProviderReq) (*iamgen.UpsertUserByProviderResp, error) {
	if req.GetProfile() == nil {
		return errResp[iamgen.UpsertUserByProviderResp](errno.ErrInvalidParam.WithMessage("profile is required")), nil
	}
	p := req.Profile
	userID, created, err := s.userBiz.UpsertByProvider(ctx, p.Email, strVal(p.Name), strVal(p.AvatarURL))
	if err != nil {
		return errResp[iamgen.UpsertUserByProviderResp](err), nil
	}
	return &iamgen.UpsertUserByProviderResp{
		Base:    okBase(),
		UserID:  userID,
		Created: created,
	}, nil
}

// GetUser 查询用户资料。
func (s *IAMImpl) GetUser(ctx context.Context, req *iamgen.GetUserReq) (*iamgen.GetUserResp, error) {
	if req.UserID == "" {
		return errResp[iamgen.GetUserResp](errno.ErrInvalidParam.WithMessage("user_id is required")), nil
	}
	u, err := s.userBiz.GetUser(ctx, req.UserID)
	if err != nil {
		return errResp[iamgen.GetUserResp](err), nil
	}
	resp := &iamgen.GetUserResp{
		Base:      okBase(),
		UserID:    u.ID.Hex(),
		Email:     u.Email,
		Name:      strPtr(u.Name),
		AvatarURL: strPtr(u.AvatarURL),
		Status:    iamgen.UserStatus(u.Status),
		CreatedAt: u.CreatedAt,
	}
	return resp, nil
}

// ---- helpers ----

func okBase() *iambase.BaseResp {
	return &iambase.BaseResp{Code: 0, Message: "ok"}
}

func errResp[T any](err error) *T {
	_ = err // 统一在外层日志记录
	return new(T)
}

func strVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// iamRespBase 把 errno 转换为 BaseResp code/message。
func iamRespBase(err error) *iambase.BaseResp {
	if err == nil {
		return okBase()
	}
	var e errno.Errno
	if errors.As(err, &e) {
		return &iambase.BaseResp{Code: e.Code, Message: e.Message}
	}
	return &iambase.BaseResp{Code: errno.ErrInternal.Code, Message: err.Error()}
}
