package main

import (
	"context"
	"errors"

	"github.com/castlexu/micro-service/pkg/errno"
	"github.com/castlexu/micro-service/services/iam/biz"
	iammodel "github.com/castlexu/micro-service/services/iam/dal/model"
	iambase "github.com/castlexu/micro-service/services/iam/kitex_gen/base"
	iamgen "github.com/castlexu/micro-service/services/iam/kitex_gen/iam"
)

// IAMImpl 实现 Kitex 生成的 IAMService 接口。
type IAMImpl struct {
	userBiz *biz.UserBiz
	roleBiz *biz.RoleBiz
	permBiz *biz.PermissionBiz
}

// NewIAMImpl 构造 IAMImpl。
func NewIAMImpl(userBiz *biz.UserBiz, roleBiz *biz.RoleBiz, permBiz *biz.PermissionBiz) *IAMImpl {
	return &IAMImpl{userBiz: userBiz, roleBiz: roleBiz, permBiz: permBiz}
}

// ---- User ----

func (s *IAMImpl) UpsertUserByProvider(ctx context.Context, req *iamgen.UpsertUserByProviderReq) (*iamgen.UpsertUserByProviderResp, error) {
	if req.GetProfile() == nil {
		return &iamgen.UpsertUserByProviderResp{Base: errBase(errno.ErrInvalidParam.WithMessage("profile is required"))}, nil
	}
	p := req.Profile
	userID, role, status, created, err := s.userBiz.UpsertByProvider(ctx, p.Email, strVal(p.Name), strVal(p.AvatarURL))
	if err != nil {
		return &iamgen.UpsertUserByProviderResp{Base: errBase(err)}, nil
	}
	return &iamgen.UpsertUserByProviderResp{
		Base:    okBase(),
		UserID:  userID,
		Created: created,
		Role:    role,
		Status:  int32(status),
	}, nil
}

func (s *IAMImpl) GetUser(ctx context.Context, req *iamgen.GetUserReq) (*iamgen.GetUserResp, error) {
	if req.UserID == "" {
		return &iamgen.GetUserResp{Base: errBase(errno.ErrInvalidParam.WithMessage("user_id is required"))}, nil
	}
	u, err := s.userBiz.GetUser(ctx, req.UserID)
	if err != nil {
		return &iamgen.GetUserResp{Base: errBase(err)}, nil
	}
	return &iamgen.GetUserResp{
		Base:      okBase(),
		UserID:    u.ID.Hex(),
		Email:     u.Email,
		Name:      strPtr(u.Name),
		AvatarURL: strPtr(u.AvatarURL),
		Phone:     strPtr(u.Phone),
		Status:    iamgen.UserStatus(u.Status),
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}, nil
}

func (s *IAMImpl) ListUsers(ctx context.Context, req *iamgen.ListUsersReq) (*iamgen.ListUsersResp, error) {
	var statusPtr *iammodel.UserStatus
	if req.Status != nil {
		st := iammodel.UserStatus(*req.Status)
		statusPtr = &st
	}
	role := ""
	if req.Role != nil {
		role = *req.Role
	}
	users, total, err := s.userBiz.ListUsers(ctx, int(req.Page), int(req.PageSize), role, statusPtr)
	if err != nil {
		return &iamgen.ListUsersResp{Base: errBase(err)}, nil
	}
	items := make([]*iamgen.UserItem, 0, len(users))
	for _, u := range users {
		items = append(items, &iamgen.UserItem{
			UserID:    u.ID.Hex(),
			Email:     u.Email,
			Name:      strPtr(u.Name),
			AvatarURL: strPtr(u.AvatarURL),
			Phone:     strPtr(u.Phone),
			Role:      u.Role,
			Status:    iamgen.UserStatus(u.Status),
			CreatedAt: u.CreatedAt,
		})
	}
	return &iamgen.ListUsersResp{Base: okBase(), Users: items, Total: total}, nil
}

func (s *IAMImpl) UpdateUserRole(ctx context.Context, req *iamgen.UpdateUserRoleReq) (*iamgen.UpdateUserRoleResp, error) {
	if err := s.userBiz.UpdateUserRole(ctx, req.TargetUserID, req.Role, req.OperatorUserID); err != nil {
		return &iamgen.UpdateUserRoleResp{Base: errBase(err)}, nil
	}
	return &iamgen.UpdateUserRoleResp{Base: okBase()}, nil
}

func (s *IAMImpl) UpdateUserStatus(ctx context.Context, req *iamgen.UpdateUserStatusReq) (*iamgen.UpdateUserStatusResp, error) {
	if err := s.userBiz.UpdateUserStatus(ctx, req.TargetUserID, iammodel.UserStatus(req.Status)); err != nil {
		return &iamgen.UpdateUserStatusResp{Base: errBase(err)}, nil
	}
	return &iamgen.UpdateUserStatusResp{Base: okBase()}, nil
}

// ---- Role ----

func (s *IAMImpl) ListRoles(ctx context.Context, req *iamgen.ListRolesReq) (*iamgen.ListRolesResp, error) {
	roles, err := s.roleBiz.ListRoles(ctx)
	if err != nil {
		return &iamgen.ListRolesResp{Base: errBase(err)}, nil
	}
	items := make([]*iamgen.RoleItem, 0, len(roles))
	for _, r := range roles {
		items = append(items, &iamgen.RoleItem{
			RoleID:      r.ID.Hex(),
			Name:        r.Name,
			DisplayName: r.DisplayName,
			Permissions: r.Permissions,
			IsSystem:    r.IsSystem,
		})
	}
	return &iamgen.ListRolesResp{Base: okBase(), Roles: items}, nil
}

func (s *IAMImpl) CreateRole(ctx context.Context, req *iamgen.CreateRoleReq) (*iamgen.CreateRoleResp, error) {
	role, err := s.roleBiz.CreateRole(ctx, req.Name, req.DisplayName, req.Permissions)
	if err != nil {
		return &iamgen.CreateRoleResp{Base: errBase(err)}, nil
	}
	return &iamgen.CreateRoleResp{
		Base: okBase(),
		Role: &iamgen.RoleItem{
			RoleID:      role.ID.Hex(),
			Name:        role.Name,
			DisplayName: role.DisplayName,
			Permissions: role.Permissions,
			IsSystem:    role.IsSystem,
		},
	}, nil
}

func (s *IAMImpl) UpdateRole(ctx context.Context, req *iamgen.UpdateRoleReq) (*iamgen.UpdateRoleResp, error) {
	if err := s.roleBiz.UpdateRole(ctx, req.RoleID, req.DisplayName, req.Permissions); err != nil {
		return &iamgen.UpdateRoleResp{Base: errBase(err)}, nil
	}
	return &iamgen.UpdateRoleResp{Base: okBase()}, nil
}

func (s *IAMImpl) DeleteRole(ctx context.Context, req *iamgen.DeleteRoleReq) (*iamgen.DeleteRoleResp, error) {
	if err := s.roleBiz.DeleteRole(ctx, req.RoleID); err != nil {
		return &iamgen.DeleteRoleResp{Base: errBase(err)}, nil
	}
	return &iamgen.DeleteRoleResp{Base: okBase()}, nil
}

func (s *IAMImpl) GetRolePermissions(ctx context.Context, req *iamgen.GetRolePermissionsReq) (*iamgen.GetRolePermissionsResp, error) {
	perms, err := s.roleBiz.GetRolePermissions(ctx, req.RoleName)
	if err != nil {
		return &iamgen.GetRolePermissionsResp{Base: errBase(err)}, nil
	}
	return &iamgen.GetRolePermissionsResp{Base: okBase(), Permissions: perms}, nil
}

// ---- Permission ----

func (s *IAMImpl) ListPermissions(ctx context.Context, req *iamgen.ListPermissionsReq) (*iamgen.ListPermissionsResp, error) {
	perms, err := s.permBiz.ListPermissions(ctx)
	if err != nil {
		return &iamgen.ListPermissionsResp{Base: errBase(err)}, nil
	}
	items := make([]*iamgen.PermissionItem, 0, len(perms))
	for _, p := range perms {
		items = append(items, &iamgen.PermissionItem{
			Code:        p.Code,
			DisplayName: p.DisplayName,
			Description: p.Description,
			IsSystem:    p.IsSystem,
		})
	}
	return &iamgen.ListPermissionsResp{Base: okBase(), Permissions: items}, nil
}

func (s *IAMImpl) CreatePermission(ctx context.Context, req *iamgen.CreatePermissionReq) (*iamgen.CreatePermissionResp, error) {
	p, err := s.permBiz.CreatePermission(ctx, req.Code, req.DisplayName, req.Description)
	if err != nil {
		return &iamgen.CreatePermissionResp{Base: errBase(err)}, nil
	}
	return &iamgen.CreatePermissionResp{
		Base: okBase(),
		Permission: &iamgen.PermissionItem{
			Code:        p.Code,
			DisplayName: p.DisplayName,
			Description: p.Description,
			IsSystem:    p.IsSystem,
		},
	}, nil
}

// ---- helpers ----

func okBase() *iambase.BaseResp {
	return &iambase.BaseResp{Code: 0, Message: "ok"}
}

func errBase(err error) *iambase.BaseResp {
	var e errno.Errno
	if errors.As(err, &e) {
		return &iambase.BaseResp{Code: e.Code, Message: e.Message}
	}
	return &iambase.BaseResp{Code: errno.ErrInternal.Code, Message: err.Error()}
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
