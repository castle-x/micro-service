package mongo

// RoleMongo 封装 roles 集合的 CRUD。
type RoleMongo struct{}

// NewRoleMongo 构造 RoleMongo。
func NewRoleMongo() *RoleMongo { return &RoleMongo{} }
