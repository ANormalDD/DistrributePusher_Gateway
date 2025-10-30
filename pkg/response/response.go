package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// StandardResponse is the unified response envelope
type StandardResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// ReplySuccess sends a 200 OK with message only
func ReplySuccess(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, StandardResponse{Code: 0, Msg: msg})
}

// ReplySuccessWithData sends a 200 OK with message and data payload (msg at top-level)
func ReplySuccessWithData(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, StandardResponse{Code: 0, Msg: msg, Data: data})
}

// ReplyBadRequest sends a 400 Bad Request with error message
func ReplyBadRequest(c *gin.Context, msg string) {
	c.JSON(http.StatusBadRequest, StandardResponse{Code: 400, Msg: msg})
}

// ReplyUnauthorized sends a 401 Unauthorized with error message
func ReplyUnauthorized(c *gin.Context, msg string) {
	c.JSON(http.StatusUnauthorized, StandardResponse{Code: 401, Msg: msg})
}

// ReplyError500 sends a 500 Internal Server Error with error message
func ReplyError500(c *gin.Context, msg string) {
	c.JSON(http.StatusInternalServerError, StandardResponse{Code: 500, Msg: msg})
}
