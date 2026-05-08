package db

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/castlexu/micro-service/pkg/utils"
)

// Repository 是一个面向 Collection + 文档类型 T 的通用仓储。
//
//   - T 必须是实现 BaseDocument 的**指针**的 Elem，例如 *User 实现 BaseDocument 时
//     Repository[User] 即可；Repository 内部会对 *T 调用接口方法。
//   - 自动注入 deleted_at 软删除过滤；如需查已删文档，FindOptions.IncludeDeleted = true。
//   - 自动维护 CreatedAt / UpdatedAt：InsertOne 前调 SetTimestamps；UpdateMap 会注入 updated_at。
//
// 典型使用：
//
//	repo := db.NewRepository[User](client, "users")
//	id, _ := repo.InsertOne(ctx, &User{Username: "alice"})
//	u, _ := repo.FindOne(ctx, bson.D{{Key: "username", Value: "alice"}})
type Repository[T any] struct {
	coll *mongo.Collection
}

// NewRepository 构造仓储实例，指向 client 默认 Database 下的 name 集合。
func NewRepository[T any](client *Client, name string) *Repository[T] {
	return &Repository[T]{coll: client.Collection(name)}
}

// Collection 返回底层 *mongo.Collection，便于执行本 Repository 未覆盖的高级操作。
func (r *Repository[T]) Collection() *mongo.Collection { return r.coll }

// asBaseDoc 将 *T 视为 BaseDocument。
// 若 T 未实现 BaseDocument，会在运行时 panic；通过测试和编码约定避免。
func asBaseDoc(ptr any) BaseDocument {
	if d, ok := ptr.(BaseDocument); ok {
		return d
	}
	panic("db: document type must implement BaseDocument (consider embedding db.BaseDoc)")
}

// ---- 读操作 ----

// FindOne 根据 filter 查找单条，返回 nil 错误表示命中。
// 未命中返回 mongo.ErrNoDocuments（可用 IsNotFound 判定）。
func (r *Repository[T]) FindOne(ctx context.Context, filter any, opts ...FindOptions) (*T, error) {
	o := firstFindOptions(opts)
	f := filter
	if !o.IncludeDeleted {
		f = applySoftDeleteFilter(filter)
	}

	var out T
	result := r.coll.FindOne(ctx, f, o.toMongoFindOneOptions())
	if err := result.Err(); err != nil {
		// ErrNoDocuments 原样返回（不要 wrap），业务用 errors.Is 判断。
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		return nil, errorf(err, "db: find one")
	}
	if err := result.Decode(&out); err != nil {
		return nil, errorf(err, "db: decode find one")
	}
	return &out, nil
}

// FindByID 按 ObjectID 主键查单条。
func (r *Repository[T]) FindByID(ctx context.Context, id primitive.ObjectID, opts ...FindOptions) (*T, error) {
	return r.FindOne(ctx, bson.D{{Key: "_id", Value: id}}, opts...)
}

// Find 根据 filter 查多条。
// 结果集大小受调用方控制（FindOptions.Limit），不设默认 Limit，避免踩旧实现的"默认 1 条"坑。
func (r *Repository[T]) Find(ctx context.Context, filter any, opts ...FindOptions) ([]*T, error) {
	o := firstFindOptions(opts)
	f := filter
	if !o.IncludeDeleted {
		f = applySoftDeleteFilter(filter)
	}

	cursor, err := r.coll.Find(ctx, f, o.toMongoFindOptions())
	if err != nil {
		return nil, errorf(err, "db: find")
	}
	defer cursor.Close(ctx)

	out := make([]*T, 0)
	// 用调用方 ctx，而非老实现的 context.TODO()，保证随请求取消。
	if err := cursor.All(ctx, &out); err != nil {
		return nil, errorf(err, "db: cursor all")
	}
	return out, nil
}

// Count 根据 filter 统计文档数量。
func (r *Repository[T]) Count(ctx context.Context, filter any, opts ...FindOptions) (int64, error) {
	o := firstFindOptions(opts)
	f := filter
	if !o.IncludeDeleted {
		f = applySoftDeleteFilter(filter)
	}
	// 空 filter 在 CountDocuments 下需要 bson.D{}，不能为 nil。
	if f == nil {
		f = bson.D{}
	}
	n, err := r.coll.CountDocuments(ctx, f, o.toMongoCountOptions())
	if err != nil {
		return 0, errorf(err, "db: count")
	}
	return n, nil
}

// Exists 判断 filter 至少命中一条。
func (r *Repository[T]) Exists(ctx context.Context, filter any, opts ...FindOptions) (bool, error) {
	n, err := r.Count(ctx, filter, FindOptions{
		Limit:          1,
		IncludeDeleted: firstFindOptions(opts).IncludeDeleted,
	})
	return n > 0, err
}

// ---- 写操作 ----

// InsertOne 插入一条文档。会在入库前调用 SetTimestamps(NowUnix) 填充 created_at / updated_at。
// 返回生成的 ObjectID。
func (r *Repository[T]) InsertOne(ctx context.Context, doc *T) (primitive.ObjectID, error) {
	bd := asBaseDoc(doc)
	now := utils.NowUnix()
	bd.SetTimestamps(now)

	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		return primitive.NilObjectID, errorf(err, "db: insert one")
	}
	id, ok := res.InsertedID.(primitive.ObjectID)
	if ok {
		bd.SetID(id)
	}
	return id, nil
}

// InsertMany 批量插入。为每条文档调用 SetTimestamps 后一起入库。
func (r *Repository[T]) InsertMany(ctx context.Context, docs []*T) ([]primitive.ObjectID, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	now := utils.NowUnix()
	payload := make([]any, 0, len(docs))
	for _, d := range docs {
		asBaseDoc(d).SetTimestamps(now)
		payload = append(payload, d)
	}
	res, err := r.coll.InsertMany(ctx, payload)
	if err != nil {
		return nil, errorf(err, "db: insert many")
	}
	ids := make([]primitive.ObjectID, 0, len(res.InsertedIDs))
	for _, raw := range res.InsertedIDs {
		if id, ok := raw.(primitive.ObjectID); ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// UpdateOne 以 filter 匹配一条并执行 update（常见用法 bson.D{{Key:"$set", Value: bson.D{...}}}）。
//
//   - 自动把 updated_at 注入 $set（若 update 是 bson.D/bson.M 且含有或缺少 $set 都会处理）；
//   - 自动注入 deleted_at 软删除过滤；
//   - 非 Upsert 模式下，MatchedCount == 0 返回 mongo.ErrNoDocuments（用 IsNotFound 判定）。
//
// 返回实际匹配数（MatchedCount）。
func (r *Repository[T]) UpdateOne(
	ctx context.Context,
	filter any,
	update any,
	opts ...UpdateOptions,
) (int64, error) {
	o := firstUpdateOptions(opts)

	f := applySoftDeleteFilter(filter)
	update = injectUpdatedAt(update, utils.NowUnix())

	res, err := r.coll.UpdateOne(ctx, f, update, o.toMongoUpdateOptions())
	if err != nil {
		return 0, errorf(err, "db: update one")
	}
	if !o.Upsert && res != nil && res.MatchedCount == 0 {
		return 0, mongo.ErrNoDocuments
	}
	return res.MatchedCount, nil
}

// UpdateMany 批量更新。返回匹配数。
func (r *Repository[T]) UpdateMany(
	ctx context.Context,
	filter any,
	update any,
	opts ...UpdateOptions,
) (int64, error) {
	o := firstUpdateOptions(opts)

	f := applySoftDeleteFilter(filter)
	update = injectUpdatedAt(update, utils.NowUnix())

	res, err := r.coll.UpdateMany(ctx, f, update, o.toMongoUpdateOptions())
	if err != nil {
		return 0, errorf(err, "db: update many")
	}
	return res.MatchedCount, nil
}

// FindOneAndUpdate 查询并更新单条，根据 opts.ReturnNew 决定返回新 / 旧文档。
// 未命中返回 mongo.ErrNoDocuments。
func (r *Repository[T]) FindOneAndUpdate(
	ctx context.Context,
	filter any,
	update any,
	opts ...FindAndUpdateOptions,
) (*T, error) {
	o := firstFindAndUpdateOptions(opts)

	f := applySoftDeleteFilter(filter)
	update = injectUpdatedAt(update, utils.NowUnix())

	var out T
	err := r.coll.FindOneAndUpdate(ctx, f, update, o.toMongoFindAndUpdateOptions()).Decode(&out)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		return nil, errorf(err, "db: find one and update")
	}
	return &out, nil
}

// ---- 删除 ----

// DeleteOne 软删除：将 deleted_at 置为当前 Unix 秒。
// 未命中返回 mongo.ErrNoDocuments。
func (r *Repository[T]) DeleteOne(ctx context.Context, filter any) error {
	now := utils.NowUnix()
	_, err := r.UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: bson.D{{Key: "deleted_at", Value: now}}}})
	return err
}

// DeleteMany 批量软删除。
func (r *Repository[T]) DeleteMany(ctx context.Context, filter any) (int64, error) {
	now := utils.NowUnix()
	return r.UpdateMany(ctx, filter, bson.D{{Key: "$set", Value: bson.D{{Key: "deleted_at", Value: now}}}})
}

// HardDeleteOne 物理删除一条（绕过软删除过滤）。谨慎使用。
func (r *Repository[T]) HardDeleteOne(ctx context.Context, filter any) error {
	res, err := r.coll.DeleteOne(ctx, filter)
	if err != nil {
		return errorf(err, "db: hard delete one")
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// HardDeleteMany 物理批量删除（绕过软删除过滤）。返回删除数量。
func (r *Repository[T]) HardDeleteMany(ctx context.Context, filter any) (int64, error) {
	res, err := r.coll.DeleteMany(ctx, filter)
	if err != nil {
		return 0, errorf(err, "db: hard delete many")
	}
	return res.DeletedCount, nil
}

// Drop 丢弃整个集合。仅在测试 / 迁移脚本中使用。
func (r *Repository[T]) Drop(ctx context.Context) error {
	if err := r.coll.Drop(ctx); err != nil {
		return errorf(err, "db: drop")
	}
	return nil
}

// ---- 内部工具 ----

// injectUpdatedAt 在 update document 的 $set 中注入 updated_at 字段。
//
// 支持 bson.D 和 bson.M；其它类型原样返回（调用方自己负责正确性）。
// 若 update 已含 $set 且其内已手动指定 updated_at，则尊重调用方设置不覆盖。
func injectUpdatedAt(update any, now int64) any {
	switch u := update.(type) {
	case bson.D:
		return injectUpdatedAtIntoD(u, now)
	case bson.M:
		return injectUpdatedAtIntoM(u, now)
	default:
		return update
	}
}

func injectUpdatedAtIntoD(u bson.D, now int64) bson.D {
	for i, elem := range u {
		if elem.Key != "$set" {
			continue
		}
		switch sv := elem.Value.(type) {
		case bson.D:
			if containsKey(sv, "updated_at") {
				return u
			}
			u[i].Value = append(sv, bson.E{Key: "updated_at", Value: now})
			return u
		case bson.M:
			if _, ok := sv["updated_at"]; !ok {
				sv["updated_at"] = now
				u[i].Value = sv
			}
			return u
		}
	}
	// 没有 $set，追加一个
	return append(u, bson.E{Key: "$set", Value: bson.D{{Key: "updated_at", Value: now}}})
}

func injectUpdatedAtIntoM(u bson.M, now int64) bson.M {
	set, ok := u["$set"]
	if !ok {
		u["$set"] = bson.M{"updated_at": now}
		return u
	}
	switch sv := set.(type) {
	case bson.M:
		if _, ok := sv["updated_at"]; !ok {
			sv["updated_at"] = now
		}
	case bson.D:
		if !containsKey(sv, "updated_at") {
			u["$set"] = append(sv, bson.E{Key: "updated_at", Value: now})
		}
	}
	return u
}

func containsKey(d bson.D, key string) bool {
	for _, e := range d {
		if e.Key == key {
			return true
		}
	}
	return false
}

// _ 保留 time 包引用避免未来扩展时遗忘。
var _ = time.Second
