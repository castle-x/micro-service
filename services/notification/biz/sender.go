package biz

// SenderBiz 封装多渠道发送（短信 / 邮件 / 站内信 / APP 推送）。
type SenderBiz struct{}

// NewSenderBiz 构造 SenderBiz。
func NewSenderBiz() *SenderBiz { return &SenderBiz{} }
