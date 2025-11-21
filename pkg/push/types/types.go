package types

import (
	"encoding/json"
	"fmt"
)

// PushMessage 是推送到消息队列的消息格式，同时也是center与gateway之间转发的消息格式
type PushMessage struct {
	ID        int64       `json:"id"`
	Type      string      `json:"type"`
	RoomID    int64       `json:"room_id"`
	SenderID  int64       `json:"sender_id"`
	TargetIDs []int64     `json:"target_ids"`
	Payload   interface{} `json:"payload"`
}

// UnmarshalJSON implements a tolerant unmarshaler that accepts both
// snake_case JSON keys (e.g. "room_id") and CamelCase keys from the
// center server (e.g. "RoomID"). It reads into a map first and then
// fills fields from available keys.
func (p *PushMessage) UnmarshalJSON(data []byte) error {
	// try the normal unmarshal first for best performance
	type alias PushMessage
	var a alias
	if err := json.Unmarshal(data, &a); err == nil {
		*p = PushMessage(a)
		// If TargetIDs is present or other fields parsed, accept it.
		// But even if some fields are zero (because tags didn't match),
		// we'll continue to the tolerant path to fill missing ones.
	}

	// tolerant path: unmarshal into map[string]json.RawMessage
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("pushmessage: invalid json: %w", err)
	}

	// helper to read keys in either snake_case or CamelCase
	getRaw := func(keys ...string) (json.RawMessage, bool) {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				return v, true
			}
		}
		return nil, false
	}

	// fill ID
	if v, ok := getRaw("id", "ID"); ok {
		var x int64
		if err := json.Unmarshal(v, &x); err == nil {
			p.ID = x
		}
	}
	// fill Type
	if v, ok := getRaw("type", "Type"); ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			p.Type = s
		}
	}
	// fill RoomID
	if v, ok := getRaw("room_id", "RoomID"); ok {
		var x int64
		if err := json.Unmarshal(v, &x); err == nil {
			p.RoomID = x
		}
	}
	// fill SenderID
	if v, ok := getRaw("sender_id", "SenderID"); ok {
		var x int64
		if err := json.Unmarshal(v, &x); err == nil {
			p.SenderID = x
		}
	}
	// fill TargetIDs
	if v, ok := getRaw("target_ids", "TargetIDs"); ok {
		var arr []int64
		if err := json.Unmarshal(v, &arr); err == nil {
			p.TargetIDs = arr
		}
	}
	// fill Payload
	if v, ok := getRaw("payload", "Payload"); ok {
		var obj interface{}
		if err := json.Unmarshal(v, &obj); err == nil {
			p.Payload = obj
		}
	}

	return nil
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
