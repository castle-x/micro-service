package biz

// TransactionBiz 处理积分流水业务（消耗 / 赚取），需防并发超卖与幂等。
type TransactionBiz struct{}

// NewTransactionBiz 构造 TransactionBiz。
func NewTransactionBiz() *TransactionBiz { return &TransactionBiz{} }
