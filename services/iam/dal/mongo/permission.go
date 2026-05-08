package mongo

// PermissionMongo 封装 permissions 集合的 CRUD。
type PermissionMongo struct{}

// NewPermissionMongo 构造 PermissionMongo。
func NewPermissionMongo() *PermissionMongo { return &PermissionMongo{} }
