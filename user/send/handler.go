package send

import (
	"Gateway/pkg/response"
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// 使用 RawMessage 接收 content，后续按 type 字段反序列化为具体 ChatPayload
type SendMessageRequest struct {
	RoomID  int64           `json:"room_id" binding:"required"`
	Content json.RawMessage `json:"content" binding:"required"`
}

type RecallMessageRequest struct {
	MessageID int64 `json:"message_id" binding:"required"`
}

func SendMessageHandler(c *gin.Context) {
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ReplyBadRequest(c, "Invalid request")
		return
	}
	id, exists := c.Get("userID")
	if !exists {
		response.ReplyUnauthorized(c, "Unauthorized")
		return
	}
	userID := id.(int64)

	// 将 raw content 解析为具体的 ChatPayload
	payload, err := UnmarshalChatPayload(req.Content)
	if err != nil {
		response.ReplyBadRequest(c, "Invalid content: "+err.Error())
		return
	}

	err = SendMessage(req.RoomID, userID, payload)
	if err != nil {
		response.ReplyError500(c, err.Error())
		return
	}
	response.ReplySuccess(c, "Message sent successfully")

}
