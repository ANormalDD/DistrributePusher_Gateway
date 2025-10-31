package send

import (
	"Gateway/pkg/db/mysql"
	"encoding/json"
	"time"
)

func InsertMessage(roomID int64, senderID int64, content ChatPayload) (int64, error) {
	query := "INSERT INTO chat_messages (room_id, sender_id, type, content, created_at) VALUES (?, ?, ?, ?, ?)"
	contentData, err := json.Marshal(content)
	if err != nil {
		return 0, err
	}
	result, err := mysql.DB.Exec(query, roomID, senderID, content.GetType(), contentData, time.Now())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
