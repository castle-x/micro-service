// Package handler 只做参数校验 + 调 RPC + 组装响应，禁止直接访问数据库。
package handler

import (
	"context"
	"net/http"
	"strconv"

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

// AdminHandler 处理 /api/v1/admin/* 路由。
type AdminHandler struct {
	iamClient iamclient.Client
	idpClient idpclient.Client
}

// NewAdminHandler 构造 AdminHandler。
func NewAdminHandler(iamClient iamclient.Client, idpClient idpclient.Client) *AdminHandler {
	return &AdminHandler{iamClient: iamClient, idpClient: idpClient}
}

// ---- 请求/响应结构 ----

type updateUserRoleReq struct {
	Role string `json:"role"`
}

type updateUserStatusReq struct {
	Status int32 `json:"status"` // 1=active 2=disabled 3=banned
}

type createRoleReq struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"`
}

type updateRoleReq struct {
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"`
}

type createPermissionReq struct {
	Code        string `json:"code"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// ---- User 管理 ----

// ListUsers GET /api/v1/admin/users
func (h *AdminHandler) ListUsers(c context.Context, ctx *app.RequestContext) {
	page, _ := strconv.Atoi(string(ctx.Query("page")))
	pageSize, _ := strconv.Atoi(string(ctx.Query("page_size")))
	role := string(ctx.Query("role"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	req := &iamgen.ListUsersReq{
		Base:     &edgebase.BaseReq{},
		Page:     int32(page),
		PageSize: int32(pageSize),
	}
	if role != "" {
		req.Role = &role
	}

	resp, err := h.iamClient.ListUsers(c, req)
	if err != nil {
		logger.Ctx(c).Error("iam.ListUsers failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if resp.Base != nil && resp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(resp.Base.Code), apiResp{Code: resp.Base.Code, Message: resp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok", Data: map[string]any{
		"users": resp.Users,
		"total": resp.Total,
	}})
}

// UpdateUserRole PUT /api/v1/admin/users/:id/role
func (h *AdminHandler) UpdateUserRole(c context.Context, ctx *app.RequestContext) {
	targetID := ctx.Param("id")
	operatorID := getOperatorID(ctx)
	var req updateUserRoleReq
	if err := ctx.BindJSON(&req); err != nil || req.Role == "" {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "role is required"})
		return
	}

	resp, err := h.iamClient.UpdateUserRole(c, &iamgen.UpdateUserRoleReq{
		Base:           &edgebase.BaseReq{},
		TargetUserID:   targetID,
		Role:           req.Role,
		OperatorUserID: operatorID,
	})
	if err != nil {
		logger.Ctx(c).Error("iam.UpdateUserRole failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if resp.Base != nil && resp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(resp.Base.Code), apiResp{Code: resp.Base.Code, Message: resp.Base.Message})
		return
	}

	// 撤销目标用户所有 refresh token，强制重新登录获取新 role
	_, _ = h.idpClient.RevokeUserTokens(c, &idpgen.RevokeUserTokensReq{
		Base:   &edgebase.BaseReq{},
		UserID: targetID,
	})

	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok"})
}

// UpdateUserStatus PUT /api/v1/admin/users/:id/status
func (h *AdminHandler) UpdateUserStatus(c context.Context, ctx *app.RequestContext) {
	targetID := ctx.Param("id")
	operatorID := getOperatorID(ctx)
	var req updateUserStatusReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	status := iamgen.UserStatus(req.Status)

	resp, err := h.iamClient.UpdateUserStatus(c, &iamgen.UpdateUserStatusReq{
		Base:           &edgebase.BaseReq{},
		TargetUserID:   targetID,
		Status:         status,
		OperatorUserID: operatorID,
	})
	if err != nil {
		logger.Ctx(c).Error("iam.UpdateUserStatus failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if resp.Base != nil && resp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(resp.Base.Code), apiResp{Code: resp.Base.Code, Message: resp.Base.Message})
		return
	}

	// 封禁(3)/禁用(2) → 撤销所有 token + 写封禁标记；解封(1) → 移除封禁标记
	if req.Status == 3 || req.Status == 2 {
		_, _ = h.idpClient.RevokeUserTokens(c, &idpgen.RevokeUserTokensReq{
			Base:   &edgebase.BaseReq{},
			UserID: targetID,
		})
	}
	// banned(3) 额外写封禁标记，让存量 access token 立即失效
	if req.Status == 3 {
		_, _ = h.idpClient.BanUser(c, &idpgen.BanUserReq{
			Base:   &edgebase.BaseReq{},
			UserID: targetID,
		})
	} else {
		// 解封或禁用改为其他状态时移除封禁标记
		_, _ = h.idpClient.UnbanUser(c, &idpgen.UnbanUserReq{
			Base:   &edgebase.BaseReq{},
			UserID: targetID,
		})
	}

	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok"})
}

// ---- Role 管理 ----

// ListRoles GET /api/v1/admin/roles
func (h *AdminHandler) ListRoles(c context.Context, ctx *app.RequestContext) {
	resp, err := h.iamClient.ListRoles(c, &iamgen.ListRolesReq{Base: &edgebase.BaseReq{}})
	if err != nil {
		logger.Ctx(c).Error("iam.ListRoles failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if resp.Base != nil && resp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(resp.Base.Code), apiResp{Code: resp.Base.Code, Message: resp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok", Data: resp.Roles})
}

// CreateRole POST /api/v1/admin/roles
func (h *AdminHandler) CreateRole(c context.Context, ctx *app.RequestContext) {
	var req createRoleReq
	if err := ctx.BindJSON(&req); err != nil || req.Name == "" {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "name is required"})
		return
	}
	resp, err := h.iamClient.CreateRole(c, &iamgen.CreateRoleReq{
		Base:           &edgebase.BaseReq{},
		Name:           req.Name,
		DisplayName:    req.DisplayName,
		Permissions:    req.Permissions,
		OperatorUserID: getOperatorID(ctx),
	})
	if err != nil {
		logger.Ctx(c).Error("iam.CreateRole failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if resp.Base != nil && resp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(resp.Base.Code), apiResp{Code: resp.Base.Code, Message: resp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok", Data: resp.Role})
}

// UpdateRole PUT /api/v1/admin/roles/:id
func (h *AdminHandler) UpdateRole(c context.Context, ctx *app.RequestContext) {
	roleID := ctx.Param("id")
	var req updateRoleReq
	if err := ctx.BindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "invalid request body"})
		return
	}
	resp, err := h.iamClient.UpdateRole(c, &iamgen.UpdateRoleReq{
		Base:           &edgebase.BaseReq{},
		RoleID:         roleID,
		DisplayName:    req.DisplayName,
		Permissions:    req.Permissions,
		OperatorUserID: getOperatorID(ctx),
	})
	if err != nil {
		logger.Ctx(c).Error("iam.UpdateRole failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if resp.Base != nil && resp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(resp.Base.Code), apiResp{Code: resp.Base.Code, Message: resp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok"})
}

// DeleteRole DELETE /api/v1/admin/roles/:id
func (h *AdminHandler) DeleteRole(c context.Context, ctx *app.RequestContext) {
	roleID := ctx.Param("id")
	resp, err := h.iamClient.DeleteRole(c, &iamgen.DeleteRoleReq{
		Base:           &edgebase.BaseReq{},
		RoleID:         roleID,
		OperatorUserID: getOperatorID(ctx),
	})
	if err != nil {
		logger.Ctx(c).Error("iam.DeleteRole failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if resp.Base != nil && resp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(resp.Base.Code), apiResp{Code: resp.Base.Code, Message: resp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok"})
}

// ---- Permission 管理 ----

// ListPermissions GET /api/v1/admin/permissions
func (h *AdminHandler) ListPermissions(c context.Context, ctx *app.RequestContext) {
	resp, err := h.iamClient.ListPermissions(c, &iamgen.ListPermissionsReq{Base: &edgebase.BaseReq{}})
	if err != nil {
		logger.Ctx(c).Error("iam.ListPermissions failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if resp.Base != nil && resp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(resp.Base.Code), apiResp{Code: resp.Base.Code, Message: resp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok", Data: resp.Permissions})
}

// CreatePermission POST /api/v1/admin/permissions
func (h *AdminHandler) CreatePermission(c context.Context, ctx *app.RequestContext) {
	var req createPermissionReq
	if err := ctx.BindJSON(&req); err != nil || req.Code == "" {
		ctx.JSON(http.StatusBadRequest, apiResp{Code: errno.ErrInvalidParam.Code, Message: "code is required"})
		return
	}
	resp, err := h.iamClient.CreatePermission(c, &iamgen.CreatePermissionReq{
		Base:           &edgebase.BaseReq{},
		Code:           req.Code,
		DisplayName:    req.DisplayName,
		Description:    req.Description,
		OperatorUserID: getOperatorID(ctx),
	})
	if err != nil {
		logger.Ctx(c).Error("iam.CreatePermission failed", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, apiResp{Code: errno.ErrInternal.Code, Message: err.Error()})
		return
	}
	if resp.Base != nil && resp.Base.Code != 0 {
		ctx.JSON(bizCodeToHTTP(resp.Base.Code), apiResp{Code: resp.Base.Code, Message: resp.Base.Message})
		return
	}
	ctx.JSON(http.StatusOK, apiResp{Code: 0, Message: "ok", Data: resp.Permission})
}

func getOperatorID(ctx *app.RequestContext) string {
	v, _ := ctx.Get("auth_user_id")
	s, _ := v.(string)
	return s
}
