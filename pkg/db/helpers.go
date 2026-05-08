package db

import "go.mongodb.org/mongo-driver/mongo"

// errNoDocumentsSentinel 暴露给测试使用，避免在多个测试文件重复 import mongo。
var errNoDocumentsSentinel = mongo.ErrNoDocuments
