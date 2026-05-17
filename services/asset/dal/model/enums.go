// Package model 定义 asset 的 MongoDB 文档模型。
package model

// AssetValueKind 描述资产组成部分的值类型，与 IDL 枚举对齐。
type AssetValueKind int32

const (
	AssetValueKindUnknown AssetValueKind = 0
	AssetValueKindText    AssetValueKind = 1
	AssetValueKindMedia   AssetValueKind = 2
	AssetValueKindJSON    AssetValueKind = 3
	AssetValueKindMixed   AssetValueKind = 4
)

// AssetSource 描述资产或媒体对象来源，与 IDL 枚举对齐。
type AssetSource int32

const (
	AssetSourceUnknown    AssetSource = 0
	AssetSourceUpload     AssetSource = 1
	AssetSourceWorkflow   AssetSource = 2
	AssetSourceGeneration AssetSource = 3
	AssetSourceImport     AssetSource = 4
)

// StorageProvider 描述对象存储供应商，与 IDL 枚举对齐。
type StorageProvider int32

const (
	StorageProviderUnknown    StorageProvider = 0
	StorageProviderLocal      StorageProvider = 1
	StorageProviderAliyunOSS  StorageProvider = 2
	StorageProviderS3         StorageProvider = 3
	StorageProviderTencentCOS StorageProvider = 4
)

// URLVisibility 描述媒体对象访问 URL 的可见性，与 IDL 枚举对齐。
type URLVisibility int32

const (
	URLVisibilityUnknown URLVisibility = 0
	URLVisibilityPrivate URLVisibility = 1
	URLVisibilityPublic  URLVisibility = 2
	URLVisibilitySigned  URLVisibility = 3
)

// UploadSessionStatus 描述上传会话状态，与 IDL 枚举对齐。
type UploadSessionStatus int32

const (
	UploadSessionStatusUnknown   UploadSessionStatus = 0
	UploadSessionStatusCreated   UploadSessionStatus = 1
	UploadSessionStatusFinalized UploadSessionStatus = 2
	UploadSessionStatusExpired   UploadSessionStatus = 3
	UploadSessionStatusCancelled UploadSessionStatus = 4
)
