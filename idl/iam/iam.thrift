namespace go iam

include "../base.thrift"

// ---- 枚举 ----

enum UserStatus {
    UNKNOWN  = 0
    ACTIVE   = 1
    DISABLED = 2
    BANNED   = 3
}

// ---- UpsertUserByProvider ----
// 幂等：provider+provider_sub 已存在则更新资料并返回，不存在则创建。

struct ProviderProfile {
    1: required string Provider      // "google" / "github" 等
    2: required string ProviderSub   // provider 侧的 subject id（Google sub 字段）
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
    2: required string UserID       // iam 侧 ObjectID hex
    3: required bool   Created      // true = 新建，false = 已存在/更新
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
    7: required i64        CreatedAt  // Unix 秒
}

// ---- Service ----

service IAMService {
    UpsertUserByProviderResp UpsertUserByProvider(1: UpsertUserByProviderReq req)
    GetUserResp              GetUser             (1: GetUserReq req)
}
