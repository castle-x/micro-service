package db

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// FindOptions 是 FindOne/FindMany/Count 等查询的强类型选项。
//
// 零值即为 "不指定任何选项"；字段逐一对应 mongo.options.FindOptions 但裁剪到常用集合。
type FindOptions struct {
	// Projection 投影字段。使用 bson.D 保持字段顺序。
	Projection bson.D
	// Sort 排序。例如 bson.D{{Key: "created_at", Value: -1}}。
	Sort bson.D
	// Hint 索引提示，可传索引名 string 或 bson.D。
	Hint any
	// Limit 最大返回条数；0 表示不限制。
	Limit int64
	// Skip 跳过条数；0 表示不跳过。
	Skip int64
	// ReadPref 读偏好；nil 表示使用集合默认。
	ReadPref *readpref.ReadPref
	// IncludeDeleted 为 true 时不注入 deleted_at 过滤，查询会包含已软删除记录。
	IncludeDeleted bool
}

// toMongoFindOptions 把 FindOptions 转为 driver 的 *options.FindOptions。
func (f FindOptions) toMongoFindOptions() *options.FindOptions {
	o := options.Find()
	if f.Projection != nil {
		o.SetProjection(f.Projection)
	}
	if f.Sort != nil {
		o.SetSort(f.Sort)
	}
	if f.Hint != nil {
		o.SetHint(f.Hint)
	}
	if f.Limit > 0 {
		o.SetLimit(f.Limit)
	}
	if f.Skip > 0 {
		o.SetSkip(f.Skip)
	}
	return o
}

// toMongoFindOneOptions 把 FindOptions 转为 driver 的 *options.FindOneOptions。
func (f FindOptions) toMongoFindOneOptions() *options.FindOneOptions {
	o := options.FindOne()
	if f.Projection != nil {
		o.SetProjection(f.Projection)
	}
	if f.Sort != nil {
		o.SetSort(f.Sort)
	}
	if f.Hint != nil {
		o.SetHint(f.Hint)
	}
	if f.Skip > 0 {
		o.SetSkip(f.Skip)
	}
	return o
}

// toMongoCountOptions 把 FindOptions 转为 driver 的 *options.CountOptions。
func (f FindOptions) toMongoCountOptions() *options.CountOptions {
	o := options.Count()
	if f.Hint != nil {
		o.SetHint(f.Hint)
	}
	if f.Limit > 0 {
		o.SetLimit(f.Limit)
	}
	if f.Skip > 0 {
		o.SetSkip(f.Skip)
	}
	return o
}

// UpdateOptions 是 UpdateOne/UpdateMany 的强类型选项。
type UpdateOptions struct {
	Upsert       bool
	ArrayFilters []any
}

func (u UpdateOptions) toMongoUpdateOptions() *options.UpdateOptions {
	o := options.Update()
	if u.Upsert {
		o.SetUpsert(true)
	}
	if len(u.ArrayFilters) > 0 {
		o.SetArrayFilters(options.ArrayFilters{Filters: u.ArrayFilters})
	}
	return o
}

// FindAndUpdateOptions 是 FindOneAndUpdate 的强类型选项。
type FindAndUpdateOptions struct {
	Projection   bson.D
	Sort         bson.D
	Upsert       bool
	ReturnNew    bool // true => 返回修改后文档
	ArrayFilters []any
}

func (f FindAndUpdateOptions) toMongoFindAndUpdateOptions() *options.FindOneAndUpdateOptions {
	o := options.FindOneAndUpdate()
	if f.Projection != nil {
		o.SetProjection(f.Projection)
	}
	if f.Sort != nil {
		o.SetSort(f.Sort)
	}
	if f.Upsert {
		o.SetUpsert(true)
	}
	if f.ReturnNew {
		o.SetReturnDocument(options.After)
	}
	if len(f.ArrayFilters) > 0 {
		o.SetArrayFilters(options.ArrayFilters{Filters: f.ArrayFilters})
	}
	return o
}

// firstFindOptions 从可变参数中取第一个，返回零值兜底。
// 推荐调用方最多传 1 个 options。
func firstFindOptions(opts []FindOptions) FindOptions {
	if len(opts) == 0 {
		return FindOptions{}
	}
	return opts[0]
}

func firstUpdateOptions(opts []UpdateOptions) UpdateOptions {
	if len(opts) == 0 {
		return UpdateOptions{}
	}
	return opts[0]
}

func firstFindAndUpdateOptions(opts []FindAndUpdateOptions) FindAndUpdateOptions {
	if len(opts) == 0 {
		return FindAndUpdateOptions{}
	}
	return opts[0]
}
