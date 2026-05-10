namespace go iam

include "../base.thrift"

// ---- 枚举 ----

enum UserStatus {
    UNKNOWN  = 0
    ACTIVE   = 1
    DISABLED = 2
    BANNED   = 3
}

enum UserSource {
    UNKNOWN       = 0
    PASSWORD      = 1
    GOOGLE        = 2
    ALIPAY        = 3
    PHONE         = 4
    ADMIN_CREATED = 5
}

// ---- UpsertUserByProvider ----

struct ProviderProfile {
    1: required string Provider
    2: required string ProviderSub
    3: required string Email
    4: optional string Name
    5: optional string AvatarURL
}

struct UpsertUserByProviderReq {
    1: required base.BaseReq    Base
    2: required ProviderProfile Profile
}

struct UpsertUserByProviderResp {
    1: required base.BaseResp Base
    2: required string UserID
    3: required bool   Created
    4: required string Role      // 用户当前角色名
    5: required i32    Status    // 用户当前状态（1=active 2=disabled 3=banned）
}

// ---- GetUser ----

struct GetUserReq {
    1: required base.BaseReq Base
    2: required string UserID
}

struct GetUserResp {
    1: required base.BaseResp Base
    2: required string     UserID
    3: required string     Email
    4: optional string     Name
    5: optional string     AvatarURL
    6: required UserStatus Status
    7: required i64        CreatedAt
    8: required string     Role
    9: optional string     Phone
}

// ---- ListUsers ----

struct ListUsersReq {
    1: required base.BaseReq Base
    2: required i32          Page      // 从 1 开始
    3: required i32          PageSize  // 最大 100
    4: optional string       Role      // 按角色过滤，空=全部
    5: optional UserStatus   Status    // 按状态过滤
}

struct UserItem {
    1: required string     UserID
    2: required string     Email
    3: optional string     Name
    4: optional string     AvatarURL
    5: required string     Role
    6: required UserStatus Status
    7: required i64        CreatedAt
    8: optional string     Phone
}

struct ListUsersResp {
    1: required base.BaseResp Base
    2: required list<UserItem> Users
    3: required i64            Total
}

// ---- UpdateUserRole ----

struct UpdateUserRoleReq {
    1: required base.BaseReq Base
    2: required string       TargetUserID
    3: required string       Role           // 新角色名
    4: required string       OperatorUserID // 操作者 user_id（用于鉴权）
}

struct UpdateUserRoleResp {
    1: required base.BaseResp Base
}

// ---- UpdateUserStatus ----

struct UpdateUserStatusReq {
    1: required base.BaseReq Base
    2: required string       TargetUserID
    3: required UserStatus   Status
    4: required string       OperatorUserID
}

struct UpdateUserStatusResp {
    1: required base.BaseResp Base
}

// ---- Role 管理 ----

struct RoleItem {
    1: required string       RoleID
    2: required string       Name
    3: required string       DisplayName
    4: required list<string> Permissions  // permission code 列表
    5: required bool         IsSystem
}

struct ListRolesReq {
    1: required base.BaseReq Base
}

struct ListRolesResp {
    1: required base.BaseResp  Base
    2: required list<RoleItem> Roles
}

struct CreateRoleReq {
    1: required base.BaseReq Base
    2: required string       Name
    3: required string       DisplayName
    4: required list<string> Permissions
    5: required string       OperatorUserID
}

struct CreateRoleResp {
    1: required base.BaseResp Base
    2: required RoleItem      Role
}

struct UpdateRoleReq {
    1: required base.BaseReq Base
    2: required string       RoleID
    3: required string       DisplayName
    4: required list<string> Permissions
    5: required string       OperatorUserID
}

struct UpdateRoleResp {
    1: required base.BaseResp Base
}

struct DeleteRoleReq {
    1: required base.BaseReq Base
    2: required string       RoleID
    3: required string       OperatorUserID
}

struct DeleteRoleResp {
    1: required base.BaseResp Base
}

// ---- Permission 管理 ----

struct PermissionItem {
    1: required string Code
    2: required string DisplayName
    3: required string Description
    4: required bool   IsSystem
}

struct ListPermissionsReq {
    1: required base.BaseReq Base
}

struct ListPermissionsResp {
    1: required base.BaseResp       Base
    2: required list<PermissionItem> Permissions
}

struct CreatePermissionReq {
    1: required base.BaseReq Base
    2: required string       Code
    3: required string       DisplayName
    4: required string       Description
    5: required string       OperatorUserID
}

struct CreatePermissionResp {
    1: required base.BaseResp  Base
    2: required PermissionItem Permission
}

// ---- GetRolePermissions（供 edge-api 鉴权用）----

struct GetRolePermissionsReq {
    1: required base.BaseReq Base
    2: required string       RoleName
}

struct GetRolePermissionsResp {
    1: required base.BaseResp Base
    2: required list<string>  Permissions // permission code 列表
}

// ---- Service ----

service IAMService {
    UpsertUserByProviderResp UpsertUserByProvider (1: UpsertUserByProviderReq req)
    GetUserResp              GetUser              (1: GetUserReq req)
    ListUsersResp            ListUsers            (1: ListUsersReq req)
    UpdateUserRoleResp       UpdateUserRole       (1: UpdateUserRoleReq req)
    UpdateUserStatusResp     UpdateUserStatus     (1: UpdateUserStatusReq req)

    ListRolesResp        ListRoles       (1: ListRolesReq req)
    CreateRoleResp       CreateRole      (1: CreateRoleReq req)
    UpdateRoleResp       UpdateRole      (1: UpdateRoleReq req)
    DeleteRoleResp       DeleteRole      (1: DeleteRoleReq req)

    ListPermissionsResp  ListPermissions  (1: ListPermissionsReq req)
    CreatePermissionResp CreatePermission (1: CreatePermissionReq req)

    GetRolePermissionsResp GetRolePermissions (1: GetRolePermissionsReq req)
}
