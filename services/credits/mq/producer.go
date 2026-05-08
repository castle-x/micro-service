package mq

// CreditEventProducer 发送积分变动事件（供 notification 订阅做站内信）。
type CreditEventProducer struct{}

// NewCreditEventProducer 构造 CreditEventProducer。
func NewCreditEventProducer() *CreditEventProducer { return &CreditEventProducer{} }
