package mongo

// TransactionMongo 封装 transactions 集合的 CRUD。
type TransactionMongo struct{}

// NewTransactionMongo 构造 TransactionMongo。
func NewTransactionMongo() *TransactionMongo { return &TransactionMongo{} }
