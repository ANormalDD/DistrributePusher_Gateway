package types

// PushMessage 是推送到消息队列的消息格式
type PushMessage struct {
	ID        int64
	Type      string
	RoomID    int64
	SenderID  int64
	TargetIDs []int64
	Payload   interface{}
}

// ClientMessage 是发送给客户端的消息格式
type ClientMessage struct {
	ID       int64       `json:"id"`
	Type     string      `json:"type"`
	RoomID   int64       `json:"room_id"`
	SenderID int64       `json:"sender_id"`
	Payload  interface{} `json:"payload"`
}

// PushTask 是推送任务，包含用户ID和序列化后的消息
type PushTask struct {
	UserID       int64
	MarshaledMsg []byte
	Msg          ClientMessage
}
