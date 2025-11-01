package center_client

import (
	"Gateway/center_client/ws"
	"Gateway/pkg/config"
	"Gateway/pkg/push/types"
	"time"

	"go.uber.org/zap"
)

type RegisterGatewayRequest struct {
	Address        string `json:"address"`
	Port           int    `json:"port"`
	MaxConnections int    `json:"max_connections"`
}

func RegisterToCenter(cfg *config.CenterConfig, gatewayAddress string, gatewayPort int, maxConnections int) error {
	// 启动 websocket 连接 goroutine（会在内部尝试重连）
	ws.Start()
	// 等待连接就绪（最多等待 10 秒）
	if err := ws.WaitReady(10 * time.Second); err != nil {
		zap.L().Error("RegisterToCenter: center ws not ready", zap.Error(err))
		return err
	}

	// 通过 websocket 发送注册消息（一次性发送，若需要可靠投递可让调用方使用 SendPushBackRequestWithRetry 或另行实现 ACK 机制）
	reqBody := RegisterGatewayRequest{
		Address:        gatewayAddress,
		Port:           gatewayPort,
		MaxConnections: maxConnections,
	}
	if err := ws.SendJSON(map[string]interface{}{
		"type": "register_gateway",
		"data": reqBody,
		"ts":   time.Now().Unix(),
	}); err != nil {
		zap.L().Error("RegisterToCenter: failed to send register via ws", zap.Error(err))
		return err
	}
	return nil
}

// 对推送失败的消息，发送到中心服务器进行转发
func SendPushBackRequest(cfg *config.CenterConfig, forwardReq types.ClientMessage) error {
	// 使用 websocket 发送 pushback 请求
	msg := map[string]interface{}{
		"type":    "pushback",
		"message": forwardReq,
		"ts":      time.Now().Unix(),
	}
	if err := ws.SendJSON(msg); err != nil {
		zap.L().Error("Failed to send pushback via ws", zap.Error(err))
		return err
	}
	return nil
}

func SendPushBackRequestWithRetry(cfg *config.CenterConfig, forwardReq types.ClientMessage, retries int, delayMs int) error {
	var err error
	for i := 0; i < retries; i++ {
		err = SendPushBackRequest(cfg, forwardReq)
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}
	return err
}

func RegisterMsg(cfg *config.CenterConfig, msg types.ClientMessage) error {
	payload := map[string]interface{}{
		"type":    "register_msg",
		"message": msg,
		"ts":      time.Now().Unix(),
	}
	if err := ws.SendJSON(payload); err != nil {
		zap.L().Error("Failed to send register_msg via ws", zap.Error(err))
		return err
	}
	return nil
}

func RegisterMsgWithRetry(cfg *config.CenterConfig, msg types.ClientMessage, retries int, delayMs int) error {
	var err error
	for i := 0; i < retries; i++ {
		err = RegisterMsg(cfg, msg)
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}
	return err
}
