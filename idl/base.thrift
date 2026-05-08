namespace go base

// 全链路透传上下文
struct BaseReq {
    1: optional string TraceID
    2: optional string Caller
    3: optional string UserID
    4: optional string TenantID
    5: optional map<string, string> Extra
}

struct PageReq {
    1: required i32 PageNum  = 1
    2: required i32 PageSize = 20
}

struct PageResp {
    1: required i32 Total
    2: required i32 PageNum
    3: required i32 PageSize
    4: required i32 TotalPages
}

struct BaseResp {
    1: required i32 Code = 0
    2: required string Message = ""
    3: optional map<string, string> Metadata
}
