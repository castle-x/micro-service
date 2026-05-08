package db

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IndexOptions 是创建索引的可选配置。
type IndexOptions struct {
	Unique                  bool
	Background              bool
	Sparse                  bool
	Name                    string
	ExpireAfterSeconds      *int32
	PartialFilterExpression bson.M
}

// IndexField 描述复合索引中的一列。
type IndexField struct {
	Name      string
	Direction any // 1 / -1 / "text" / "2d" / "2dsphere" / "hashed"
}

// parseIndexSpec 解析 "field:dir" 字符串为 IndexField。
// 支持 direction: 1, -1, asc, desc, text, 2d, 2dsphere, hashed。
// 缺省 direction 为 1（升序）。非法 direction 的字符串原样透传，
// 交给 driver 判定是否合法。
func parseIndexSpec(spec string) (IndexField, error) {
	parts := strings.SplitN(spec, ":", 2)
	if parts[0] == "" {
		return IndexField{}, fmt.Errorf("db: empty index field name in %q", spec)
	}
	f := IndexField{Name: parts[0], Direction: 1}
	if len(parts) == 1 {
		return f, nil
	}
	switch strings.ToLower(parts[1]) {
	case "1", "asc", "ascending":
		f.Direction = 1
	case "-1", "desc", "descending":
		f.Direction = -1
	case "text":
		f.Direction = "text"
	case "2d":
		f.Direction = "2d"
	case "2dsphere":
		f.Direction = "2dsphere"
	case "hashed":
		f.Direction = "hashed"
	default:
		if n, err := strconv.Atoi(parts[1]); err == nil {
			f.Direction = n
		} else {
			f.Direction = parts[1]
		}
	}
	return f, nil
}

func parseIndexSpecs(specs []string) ([]IndexField, error) {
	out := make([]IndexField, 0, len(specs))
	for _, s := range specs {
		f, err := parseIndexSpec(s)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// fieldsToKeys 把 IndexField 数组转成 bson.D，保留顺序。
func fieldsToKeys(fs []IndexField) bson.D {
	keys := make(bson.D, 0, len(fs))
	for _, f := range fs {
		keys = append(keys, bson.E{Key: f.Name, Value: f.Direction})
	}
	return keys
}

// CreateIndexes 创建索引（简化形式）。specs 示例：["user_id", "created_at:-1"]。
// unique 控制是否唯一。
func (c *Client) CreateIndexes(ctx context.Context, collection string, specs []string, unique bool) error {
	return c.CreateIndexesWithOptions(ctx, collection, specs, IndexOptions{Unique: unique})
}

// CreateIndexesWithOptions 创建索引（完整选项）。
// 该调用会阻塞直到索引创建完成；大集合请在维护窗口/低峰期执行，或在 goroutine 中启动。
func (c *Client) CreateIndexesWithOptions(
	ctx context.Context,
	collection string,
	specs []string,
	idxOpts IndexOptions,
) error {
	fields, err := parseIndexSpecs(specs)
	if err != nil {
		return err
	}
	keys := fieldsToKeys(fields)
	if len(keys) == 0 {
		return NewError("db: no index fields provided", nil)
	}
	// 单 _id 索引无需创建
	if len(keys) == 1 && keys[0].Key == "_id" {
		return nil
	}

	o := options.Index()
	if idxOpts.Unique {
		o.SetUnique(true)
	}
	if idxOpts.Background {
		o.SetBackground(true)
	}
	if idxOpts.Sparse {
		o.SetSparse(true)
	}
	if idxOpts.Name != "" {
		o.SetName(idxOpts.Name)
	}
	if idxOpts.ExpireAfterSeconds != nil {
		o.SetExpireAfterSeconds(*idxOpts.ExpireAfterSeconds)
	}
	if len(idxOpts.PartialFilterExpression) > 0 {
		o.SetPartialFilterExpression(idxOpts.PartialFilterExpression)
	}

	coll := c.Collection(collection)
	_, err = coll.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: keys, Options: o})
	if err != nil {
		return errorf(err, "db: create index on %s", collection)
	}
	return nil
}

// HasIndex 按索引名精确判断是否存在。
func (c *Client) HasIndex(ctx context.Context, collection string, name string) (bool, error) {
	cursor, err := c.Collection(collection).Indexes().List(ctx)
	if err != nil {
		return false, errorf(err, "db: list indexes %s", collection)
	}
	defer cursor.Close(ctx)
	var doc bson.M
	for cursor.Next(ctx) {
		if err := cursor.Decode(&doc); err != nil {
			return false, errorf(err, "db: decode index")
		}
		if n, ok := doc["name"].(string); ok && n == name {
			return true, nil
		}
	}
	return false, cursor.Err()
}

// HasIndexesPrefixMatch 严格按"左前缀"判断集合中是否已有可覆盖给定字段序列的复合索引。
// 符合 MongoDB 复合索引使用规则：索引 {a,b,c} 能覆盖查询 {a}、{a,b}、{a,b,c}，
// 但不能覆盖 {b}、{b,a} 等。
func (c *Client) HasIndexesPrefixMatch(ctx context.Context, collection string, specs []string) (bool, error) {
	expected, err := parseIndexSpecs(specs)
	if err != nil {
		return false, err
	}
	if len(expected) == 0 {
		return false, nil
	}

	cursor, err := c.Collection(collection).Indexes().List(ctx)
	if err != nil {
		return false, errorf(err, "db: list indexes %s", collection)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc bson.D
		if err := cursor.Decode(&doc); err != nil {
			return false, errorf(err, "db: decode index")
		}
		keys, ok := extractIndexKeys(doc)
		if !ok {
			continue
		}
		if isPrefixMatch(keys, expected) {
			return true, nil
		}
	}
	return false, cursor.Err()
}

// extractIndexKeys 从索引文档中取出 "key" 字段（bson.D 可保证顺序）。
func extractIndexKeys(doc bson.D) (primitive.D, bool) {
	for _, elem := range doc {
		if elem.Key != "key" {
			continue
		}
		if k, ok := elem.Value.(primitive.D); ok {
			return k, true
		}
	}
	return nil, false
}

// isPrefixMatch 检查 expected 是否为 existing 的左前缀，且每个位置的 direction 一致。
func isPrefixMatch(existing primitive.D, expected []IndexField) bool {
	if len(expected) > len(existing) {
		return false
	}
	for i, want := range expected {
		got := existing[i]
		if got.Key != want.Name {
			return false
		}
		if !equalIndexDirection(got.Value, want.Direction) {
			return false
		}
	}
	return true
}

// equalIndexDirection 把 int 的各种宽度统一比较，同时兼容字符串型索引（text/hashed 等）。
func equalIndexDirection(a, b any) bool {
	ai, aok := toInt64(a)
	bi, bok := toInt64(b)
	if aok && bok {
		return ai == bi
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	}
	return 0, false
}
