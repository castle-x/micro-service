namespace go idp

include "../base.thrift"

// ---- 枚举 ----

enum TokenType {
    UNKNOWN       = 0
    ACCESS_TOKEN  = 1
    REFRESH_TOKEN = 2
}

// ---- GetGoogleAuthURL ----

struct GetGoogleAuthURLReq {
    1: required base.BaseReq Base
    2: optional string RedirectURI  // 覆盖服务端默认回调地址（可选）
}

struct GetGoogleAuthURLResp {
    1: required base.BaseResp Base
    2: required string AuthURL      // 重定向给用户的 Google OAuth2 URL
    3: required string State        // 防 CSRF 随机 state（已存入 oauth_states）
}

// ---- LoginByGoogle ----

struct LoginByGoogleReq {
    1: required base.BaseReq Base
    2: required string Code         // Google 回调带回的授权码
    3: required string State        // 防 CSRF state，需与 GetGoogleAuthURL 返回值对应
    4: optional string RedirectURI  // 与请求 URL 时一致
}

struct LoginByGoogleResp {
    1: required base.BaseResp Base
    2: required string AccessToken
    3: required string RefreshToken
    4: required i64    ExpiresAt    // Unix 秒，access token 过期时间
    5: required string UserID       // iam 侧 user_id（ObjectID hex）
}

// ---- RefreshToken ----

struct RefreshTokenReq {
    1: required base.BaseReq Base
    2: required string RefreshToken
}

struct RefreshTokenResp {
    1: required base.BaseResp Base
    2: required string AccessToken
    3: required string RefreshToken // 滚动刷新
    4: required i64    ExpiresAt
}

// ---- VerifyToken ----

struct VerifyTokenReq {
    1: required base.BaseReq Base
    2: required string Token
    3: optional TokenType Type      // 不传则只校验签名/有效期，不限类型
}

struct VerifyTokenResp {
    1: required base.BaseResp Base
    2: required string UserID
    3: required string TenantID
    4: required i64    ExpiresAt
}

// ---- GetAlipayAuthURL ----

struct GetAlipayAuthURLReq {
    1: required base.BaseReq Base
    2: optional string RedirectURI
}

struct GetAlipayAuthURLResp {
    1: required base.BaseResp Base
    2: required string AuthURL
    3: required string State
}

// ---- LoginByAlipay ----

struct LoginByAlipayReq {
    1: required base.BaseReq Base
    2: required string AuthCode  // 支付宝回调的 auth_code
    3: required string State
}

struct LoginByAlipayResp {
    1: required base.BaseResp Base
    2: required string AccessToken
    3: required string RefreshToken
    4: required i64    ExpiresAt
    5: required string UserID
}

// ---- Service ----

service IDPService {
    GetGoogleAuthURLResp GetGoogleAuthURL(1: GetGoogleAuthURLReq req)
    LoginByGoogleResp    LoginByGoogle   (1: LoginByGoogleReq req)
    GetAlipayAuthURLResp GetAlipayAuthURL(1: GetAlipayAuthURLReq req)
    LoginByAlipayResp    LoginByAlipay   (1: LoginByAlipayReq req)
    RefreshTokenResp     RefreshToken    (1: RefreshTokenReq req)
    VerifyTokenResp      VerifyToken     (1: VerifyTokenReq req)
}
