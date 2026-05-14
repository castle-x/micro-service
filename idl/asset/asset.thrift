namespace go asset

include "../base.thrift"

// ---- Enums ----

enum AssetValueKind {
    UNKNOWN = 0
    TEXT    = 1
    MEDIA   = 2
    JSON    = 3
    MIXED   = 4
}

enum AssetSource {
    UNKNOWN    = 0
    UPLOAD     = 1
    WORKFLOW   = 2
    GENERATION = 3
    IMPORT     = 4
}

enum StorageProvider {
    UNKNOWN     = 0
    LOCAL       = 1
    ALIYUN_OSS  = 2
    S3          = 3
    TENCENT_COS = 4
}

enum URLVisibility {
    UNKNOWN = 0
    PRIVATE = 1
    PUBLIC  = 2
    SIGNED  = 3
}

enum UploadSessionStatus {
    UNKNOWN   = 0
    CREATED   = 1
    FINALIZED = 2
    EXPIRED   = 3
    CANCELLED = 4
}

// ---- Shared DTOs ----

struct ProvenanceDTO {
    1: optional string WorkflowRunID
    2: optional string StepRunID
    3: optional string GenerationJobID
    4: optional string PromptID
    5: optional map<string, string> Extra
}

struct AssetPartSchemaDTO {
    1: required string Key
    2: required string Name
    3: optional string Description
    4: required list<AssetValueKind> AllowedValueKinds
    5: required bool Multiple
    6: required bool Required
    7: required i32 SortOrder
}

struct AssetTypeDTO {
    1: required string AssetTypeID
    2: required string WorkspaceID
    3: required string Name
    4: required string Code
    5: optional string Description
    6: required list<AssetPartSchemaDTO> PartSchemas
    7: required string CreatedBy
    8: required i64 CreatedAt
    9: required i64 UpdatedAt
}

struct AssetDTO {
    1: required string AssetID
    2: required string WorkspaceID
    3: required string TypeID
    4: required string Name
    5: optional string Description
    6: required bool SavedToLibrary
    7: optional string CategoryID
    8: required i32 CurrentVersion
    9: optional string CoverMediaID
    10: required AssetSource Source
    11: optional ProvenanceDTO Provenance
    12: required string CreatedBy
    13: required i64 CreatedAt
    14: required i64 UpdatedAt
}

struct AssetPartValueDTO {
    1: required AssetValueKind ValueKind
    2: optional string Text
    3: optional string JSON
    4: optional list<string> MediaIDs
}

struct AssetVersionDTO {
    1: required string VersionID
    2: required string AssetID
    3: required i32 Version
    4: required map<string, AssetPartValueDTO> Parts
    5: optional string ChangeReason
    6: optional ProvenanceDTO Provenance
    7: required string CreatedBy
    8: required i64 CreatedAt
}

struct MediaVariantDTO {
    1: required string Kind
    2: required string ObjectKey
    3: optional string CDNURL
    4: optional i32 Width
    5: optional i32 Height
    6: optional i64 Size
}

struct MediaObjectDTO {
    1: required string MediaID
    2: required string WorkspaceID
    3: required StorageProvider Provider
    4: required string Bucket
    5: required string ObjectKey
    6: optional string CDNURL
    7: required URLVisibility URLVisibility
    8: required string ContentType
    9: required i64 Size
    10: optional i32 Width
    11: optional i32 Height
    12: optional string SHA256
    13: optional list<MediaVariantDTO> Variants
    14: required AssetSource Source
    15: optional ProvenanceDTO Provenance
    16: required string CreatedBy
    17: required i64 CreatedAt
}

struct AssetCategoryDTO {
    1: required string CategoryID
    2: required string WorkspaceID
    3: required string Name
    4: optional string ParentID
    5: required i32 SortOrder
    6: required string CreatedBy
    7: required i64 CreatedAt
    8: required i64 UpdatedAt
}

struct StorageUploadSessionDTO {
    1: required string SessionID
    2: required string WorkspaceID
    3: required StorageProvider Provider
    4: required string Bucket
    5: required string ObjectKey
    6: required UploadSessionStatus Status
    7: required i64 ExpiresAt
    8: required string CreatedBy
    9: required i64 CreatedAt
    10: optional i64 FinalizedAt
    11: optional string ContentType
    12: optional i64 Size
    13: optional string SHA256
    14: optional string MediaID
}

struct StoragePresignedURLDTO {
    1: required string Method
    2: required string URL
    3: optional map<string, string> Headers
    4: required i64 ExpiresAt
}

// ---- Health ----

struct HealthReq {
    1: required base.BaseReq Base
}

struct HealthResp {
    1: required base.BaseResp Base
    2: required string Service
    3: required string Status
}

// ---- AssetType CRUD ----

struct CreateAssetTypeReq {
    1: required base.BaseReq Base
    2: required string Name
    3: required string Code
    4: optional string Description
    5: required list<AssetPartSchemaDTO> PartSchemas
}

struct CreateAssetTypeResp {
    1: required base.BaseResp Base
    2: optional AssetTypeDTO AssetType
}

struct UpdateAssetTypeReq {
    1: required base.BaseReq Base
    2: required string AssetTypeID
    3: required string Name
    4: optional string Description
    5: required list<AssetPartSchemaDTO> PartSchemas
}

struct UpdateAssetTypeResp {
    1: required base.BaseResp Base
    2: optional AssetTypeDTO AssetType
}

struct GetAssetTypeReq {
    1: required base.BaseReq Base
    2: required string AssetTypeID
}

struct GetAssetTypeResp {
    1: required base.BaseResp Base
    2: optional AssetTypeDTO AssetType
}

struct ListAssetTypesReq {
    1: required base.BaseReq Base
    2: required base.PageReq Page
}

struct ListAssetTypesResp {
    1: required base.BaseResp Base
    2: required list<AssetTypeDTO> AssetTypes
    3: required base.PageResp Page
}

struct DeleteAssetTypeReq {
    1: required base.BaseReq Base
    2: required string AssetTypeID
}

struct DeleteAssetTypeResp {
    1: required base.BaseResp Base
}

// ---- Category CRUD ----

struct CreateAssetCategoryReq {
    1: required base.BaseReq Base
    2: required string Name
    3: optional string ParentID
    4: required i32 SortOrder
}

struct CreateAssetCategoryResp {
    1: required base.BaseResp Base
    2: optional AssetCategoryDTO Category
}

struct UpdateAssetCategoryReq {
    1: required base.BaseReq Base
    2: required string CategoryID
    3: required string Name
    4: optional string ParentID
    5: required i32 SortOrder
}

struct UpdateAssetCategoryResp {
    1: required base.BaseResp Base
    2: optional AssetCategoryDTO Category
}

struct ListAssetCategoriesReq {
    1: required base.BaseReq Base
}

struct ListAssetCategoriesResp {
    1: required base.BaseResp Base
    2: required list<AssetCategoryDTO> Categories
}

struct DeleteAssetCategoryReq {
    1: required base.BaseReq Base
    2: required string CategoryID
}

struct DeleteAssetCategoryResp {
    1: required base.BaseResp Base
}

// ---- Asset CRUD ----

struct CreateAssetReq {
    1: required base.BaseReq Base
    2: required string TypeID
    3: required string Name
    4: optional string Description
    5: required bool SavedToLibrary
    6: optional string CategoryID
    7: optional string CoverMediaID
    8: optional AssetSource Source
    9: optional ProvenanceDTO Provenance
}

struct CreateAssetResp {
    1: required base.BaseResp Base
    2: optional AssetDTO Asset
}

struct UpdateAssetReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required string Name
    4: optional string Description
    5: optional string CategoryID
    6: optional string CoverMediaID
    7: optional AssetSource Source
    8: optional ProvenanceDTO Provenance
}

struct UpdateAssetResp {
    1: required base.BaseResp Base
    2: optional AssetDTO Asset
}

struct GetAssetReq {
    1: required base.BaseReq Base
    2: required string AssetID
}

struct GetAssetResp {
    1: required base.BaseResp Base
    2: optional AssetDTO Asset
}

struct ListAssetsReq {
    1: required base.BaseReq Base
    2: required base.PageReq Page
    3: optional string TypeID
    4: optional string CategoryID
    5: optional bool SavedToLibrary
}

struct ListAssetsResp {
    1: required base.BaseResp Base
    2: required list<AssetDTO> Assets
    3: required base.PageResp Page
}

struct SetAssetLibraryStateReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required bool SavedToLibrary
}

struct SetAssetLibraryStateResp {
    1: required base.BaseResp Base
    2: optional AssetDTO Asset
}

struct DeleteAssetReq {
    1: required base.BaseReq Base
    2: required string AssetID
}

struct DeleteAssetResp {
    1: required base.BaseResp Base
}

// ---- Asset Version CRUD ----

struct CreateAssetVersionReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required map<string, AssetPartValueDTO> Parts
    4: optional string ChangeReason
    5: optional ProvenanceDTO Provenance
}

struct CreateAssetVersionResp {
    1: required base.BaseResp Base
    2: optional AssetDTO Asset
    3: optional AssetVersionDTO Version
}

struct CopyAssetVersionReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required i32 FromVersion
    4: optional map<string, AssetPartValueDTO> PartOverrides
    5: optional string ChangeReason
    6: optional ProvenanceDTO Provenance
}

struct CopyAssetVersionResp {
    1: required base.BaseResp Base
    2: optional AssetDTO Asset
    3: optional AssetVersionDTO Version
}

struct GetAssetVersionReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required i32 Version
}

struct GetAssetVersionResp {
    1: required base.BaseResp Base
    2: optional AssetVersionDTO Version
}

struct GetCurrentAssetVersionReq {
    1: required base.BaseReq Base
    2: required string AssetID
}

struct GetCurrentAssetVersionResp {
    1: required base.BaseResp Base
    2: optional AssetVersionDTO Version
}

struct ListAssetVersionsReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required base.PageReq Page
}

struct ListAssetVersionsResp {
    1: required base.BaseResp Base
    2: required list<AssetVersionDTO> Versions
    3: required base.PageResp Page
}

struct SetCurrentAssetVersionReq {
    1: required base.BaseReq Base
    2: required string AssetID
    3: required i32 Version
}

struct SetCurrentAssetVersionResp {
    1: required base.BaseResp Base
    2: optional AssetDTO Asset
    3: optional AssetVersionDTO Version
}

// ---- Media Storage ----

struct CreateStorageUploadSessionReq {
    1: required base.BaseReq Base
    2: required string ContentType
    3: required i64 Size
    4: optional string Filename
    5: optional string SHA256
}

struct CreateStorageUploadSessionResp {
    1: required base.BaseResp Base
    2: optional StorageUploadSessionDTO Session
    3: optional StoragePresignedURLDTO Upload
}

struct FinalizeStorageUploadSessionReq {
    1: required base.BaseReq Base
    2: required string SessionID
    3: optional string SHA256
    4: optional i32 Width
    5: optional i32 Height
}

struct FinalizeStorageUploadSessionResp {
    1: required base.BaseResp Base
    2: optional StorageUploadSessionDTO Session
    3: optional MediaObjectDTO Media
}

struct GetMediaObjectReq {
    1: required base.BaseReq Base
    2: required string MediaID
}

struct GetMediaObjectResp {
    1: required base.BaseResp Base
    2: optional MediaObjectDTO Media
}

struct ListMediaObjectsReq {
    1: required base.BaseReq Base
    2: required base.PageReq Page
    3: optional AssetSource Source
    4: optional string ContentType
}

struct ListMediaObjectsResp {
    1: required base.BaseResp Base
    2: required list<MediaObjectDTO> Media
    3: required base.PageResp Page
}

struct GetMediaObjectAccessURLReq {
    1: required base.BaseReq Base
    2: required string MediaID
    3: optional i32 ExpiresInSeconds
}

struct GetMediaObjectAccessURLResp {
    1: required base.BaseResp Base
    2: optional MediaObjectDTO Media
    3: optional StoragePresignedURLDTO Access
}

service AssetService {
    HealthResp Health(1: HealthReq req)
    CreateAssetTypeResp CreateAssetType(1: CreateAssetTypeReq req)
    UpdateAssetTypeResp UpdateAssetType(1: UpdateAssetTypeReq req)
    GetAssetTypeResp GetAssetType(1: GetAssetTypeReq req)
    ListAssetTypesResp ListAssetTypes(1: ListAssetTypesReq req)
    DeleteAssetTypeResp DeleteAssetType(1: DeleteAssetTypeReq req)
    CreateAssetCategoryResp CreateAssetCategory(1: CreateAssetCategoryReq req)
    UpdateAssetCategoryResp UpdateAssetCategory(1: UpdateAssetCategoryReq req)
    ListAssetCategoriesResp ListAssetCategories(1: ListAssetCategoriesReq req)
    DeleteAssetCategoryResp DeleteAssetCategory(1: DeleteAssetCategoryReq req)
    CreateAssetResp CreateAsset(1: CreateAssetReq req)
    UpdateAssetResp UpdateAsset(1: UpdateAssetReq req)
    GetAssetResp GetAsset(1: GetAssetReq req)
    ListAssetsResp ListAssets(1: ListAssetsReq req)
    SetAssetLibraryStateResp SetAssetLibraryState(1: SetAssetLibraryStateReq req)
    DeleteAssetResp DeleteAsset(1: DeleteAssetReq req)
    CreateAssetVersionResp CreateAssetVersion(1: CreateAssetVersionReq req)
    CopyAssetVersionResp CopyAssetVersion(1: CopyAssetVersionReq req)
    GetAssetVersionResp GetAssetVersion(1: GetAssetVersionReq req)
    GetCurrentAssetVersionResp GetCurrentAssetVersion(1: GetCurrentAssetVersionReq req)
    ListAssetVersionsResp ListAssetVersions(1: ListAssetVersionsReq req)
    SetCurrentAssetVersionResp SetCurrentAssetVersion(1: SetCurrentAssetVersionReq req)
    CreateStorageUploadSessionResp CreateStorageUploadSession(1: CreateStorageUploadSessionReq req)
    FinalizeStorageUploadSessionResp FinalizeStorageUploadSession(1: FinalizeStorageUploadSessionReq req)
    GetMediaObjectResp GetMediaObject(1: GetMediaObjectReq req)
    ListMediaObjectsResp ListMediaObjects(1: ListMediaObjectsReq req)
    GetMediaObjectAccessURLResp GetMediaObjectAccessURL(1: GetMediaObjectAccessURLReq req)
}
