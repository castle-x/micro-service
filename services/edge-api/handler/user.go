package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/pkg/logger"
	edgebase "github.com/castlexu/micro-service/services/edge-api/kitex_gen/base"
	iamgen "github.com/castlexu/micro-service/services/edge-api/kitex_gen/iam"
	iamclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/iam/iamservice"
	idpgen "github.com/castlexu/micro-service/services/edge-api/kitex_gen/idp"
	idpclient "github.com/castlexu/micro-service/services/edge-api/kitex_gen/idp/idpservice"
)

// UserHandler 处理 /api/v1/user/* 路由。
type UserHandler struct {
	idpClient idpclient.Client
	iamClient iamclient.Client
}

// NewUserHandler 构造 UserHandler。
func NewUserHandler(idpClient idpclient.Client, iamClient iamclient.Client) *UserHandler {
	return &UserHandler{idpClient: idpClient, iamClient: iamClient}
}

// meResp 是 GET /api/v1/user/me 的响应体。
type meResp struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// GetMe GET /api/v1/user/me
// 从 Authorization: Bearer <token> 头里取 access token，
// 验证后查询用户信息并返回。
func (h *UserHandler) GetMe(c context.Context, ctx *app.RequestContext) {
	// 1. 取 token
	authHeader := string(ctx.GetHeader("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		ctx.JSON(http.StatusUnauthorized, apiResp{Code: errno.ErrUnauthorized.Code, Message: "missing or invalid Authorization header"})
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// 2. 调 idp VerifyToken 拿 user_id
	verifyResp, err := h.idpClient.VerifyToken(c, &idpgen.VerifyTokenReq{
		Base:  &edgebase.BaseReq{},
		Token: token,
	})
	if err != nil {
		logger.Ctx(c).Error("idp.VerifyToken failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if verifyResp.Base != nil && verifyResp.Base.Code != 0 {
		httpCode := bizCodeToHTTP(verifyResp.Base.Code)
		ctx.JSON(httpCode, apiResp{Code: verifyResp.Base.Code, Message: verifyResp.Base.Message})
		return
	}
	userID := verifyResp.UserID

	// 3. 调 iam GetUser 拿用户信息
	getUserResp, err := h.iamClient.GetUser(c, &iamgen.GetUserReq{
		Base:   &edgebase.BaseReq{},
		UserID: userID,
	})
	if err != nil {
		logger.Ctx(c).Error("iam.GetUser failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if getUserResp.Base != nil && getUserResp.Base.Code != 0 {
		httpCode := bizCodeToHTTP(getUserResp.Base.Code)
		ctx.JSON(httpCode, apiResp{Code: getUserResp.Base.Code, Message: getUserResp.Base.Message})
		return
	}

	// 4. 返回用户信息
	ctx.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "ok",
		Data: meResp{
			UserID:    getUserResp.UserID,
			Email:     getUserResp.Email,
			Name:      getUserResp.GetName(),
			AvatarURL: getUserResp.GetAvatarURL(),
		},
	})
}
