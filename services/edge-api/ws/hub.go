// Package ws 管理 WebSocket 长连接、心跳、房间、用户绑定。
// TODO: implement per SPEC.md §2
package ws

// Hub 是所有客户端连接的中心管理器，负责广播、加入/离开房间。
type Hub struct {
	// TODO: clients map[*Client]bool; rooms map[string]*Room
}

// NewHub 创建一个 Hub 并启动后台 goroutine。
func NewHub() *Hub {
	return &Hub{}
}

// Run 启动 Hub 事件循环。
func (h *Hub) Run() {}
