package db

import "go.mongodb.org/mongo-driver/bson"

// applySoftDeleteFilter 把 {"deleted_at": {"$exists": false}} 通过 $and 合并到 filter，
// 避免与业务 filter 中已存在的 deleted_at 条件冲突。
//
//   - filter 为 nil 时，返回仅带软删除条件的 filter；
//   - filter 非空时，包装为 $and 数组，保证两个条件都生效。
//
// 注意：传入 filter 必须是 bson.D 或可隐式转换为 bson.D 的结构；
// 其他类型（如 bson.M）直接包入 $and。
func applySoftDeleteFilter(filter any) bson.D {
	notDeleted := bson.D{{Key: "deleted_at", Value: bson.D{{Key: "$exists", Value: false}}}}
	if filter == nil {
		return notDeleted
	}
	// 零长度 bson.D / bson.M 等价于 {}，直接返回 notDeleted 即可。
	switch v := filter.(type) {
	case bson.D:
		if len(v) == 0 {
			return notDeleted
		}
		return bson.D{{Key: "$and", Value: bson.A{v, notDeleted}}}
	case bson.M:
		if len(v) == 0 {
			return notDeleted
		}
		return bson.D{{Key: "$and", Value: bson.A{v, notDeleted}}}
	default:
		return bson.D{{Key: "$and", Value: bson.A{v, notDeleted}}}
	}
}
