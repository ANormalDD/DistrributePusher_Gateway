package ws

// 用于处理与中心服务器的 WebSocket 连接：连接管理、心跳与推送消息转发

import (
	"context"
	"errors"
	"sync"
	"time"

	"Gateway/pkg/config"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var (
	conn    *websocket.Conn
	cancel  context.CancelFunc
	writeMu sync.Mutex
)

// Start 连接到中心服务器并启动读取循环。会在连接断开后尝试重连。
func Start() {
	if config.Conf == nil || config.Conf.CenterConfig == nil || config.Conf.CenterConfig.Address == "" {
		zap.L().Warn("center config address empty, skipping center ws start")
		return
	}

	ctx, c := context.WithCancel(context.Background())
	cancel = c

	go func() {
		// 重连循环
		backoff := time.Second
		for {
			select {
			case <-ctx.Done():
				zap.L().Info("center ws start loop canceled")
				return
			default:
			}

			err := connectAndServe(ctx, config.Conf.CenterConfig.Address)
			if err != nil {
				zap.L().Error("center ws connection loop error", zap.Error(err))
			}
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}()
}

// Stop 停止 center ws 连接与 goroutine
func Stop() error {
	if cancel != nil {
		cancel()
	}
	if conn != nil {
		return conn.Close()
	}
	return nil
}

func connectAndServe(ctx context.Context, addr string) error {
	zap.L().Info("dialing center websocket", zap.String("addr", addr))
	d := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	c, _, err := d.Dial(addr, nil)
	if err != nil {
		return err
	}
	conn = c
	defer func() {
		_ = conn.Close()
		conn = nil
	}()

	// heartbeat settings (客户端侧)
	const (
		pongWait   = 60 * time.Second
		pingPeriod = (pongWait * 9) / 10
		writeWait  = 10 * time.Second
	)

	// 扩展读取超时的 pong handler
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// 处理服务器发来的 ping（中心将 heartbeat 通过 websocket ping 发送）
	conn.SetPingHandler(func(appData string) error {
		// extend read deadline
		conn.SetReadDeadline(time.Now().Add(pongWait))

		// reply pong (control frame) under write lock
		writeMu.Lock()
		err := conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(writeWait))
		writeMu.Unlock()
		if err != nil {
			zap.L().Warn("failed to write pong in ping handler", zap.Error(err))
			return err
		}

		return nil
	})

	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			zap.L().Warn("center ws read error", zap.Error(err))
			return err
		}

		switch mt {
		case websocket.TextMessage, websocket.BinaryMessage:
			handleIncoming(data)
		default:
		}
	}
}

func handleIncoming(data []byte) {
	zap.L().Debug("received message from center (ignored)", zap.ByteString("data", data))
}

func ReportUserConnect(userID int64) error {
	msg := map[string]interface{}{
		"type":    "user_connect",
		"user_id": userID,
		"ts":      time.Now().Unix(),
	}
	return sendJSONSafe(msg)
}

func ReportUserDisconnect(userID int64) error {
	msg := map[string]interface{}{
		"type":    "user_disconnect",
		"user_id": userID,
		"ts":      time.Now().Unix(),
	}
	return sendJSONSafe(msg)
}

func sendJSONSafe(v interface{}) error {
	if conn == nil {
		return errors.New("no center connection")
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteJSON(v); err != nil {
		return err
	}
	return nil
}
