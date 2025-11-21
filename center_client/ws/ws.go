package ws

// 用于处理与中心服务器的 WebSocket 连接：连接管理、心跳与推送消息转发

import (
	"Gateway/pkg/config"
	"Gateway/pkg/push"
	"Gateway/pkg/push/types"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var (
	conn    *websocket.Conn
	cancel  context.CancelFunc
	writeMu sync.Mutex
)

// package-level context so background reporters can observe stop signal
var ctxGlobal context.Context

// readyMu protects readyCh and connected
var (
	readyMu   sync.Mutex
	readyCh   chan struct{}
	connected bool
)

func init() {
	// initially not connected; readyCh is open and will be closed when connected
	readyCh = make(chan struct{})
	connected = false
}

// Start 连接到中心服务器并启动读取循环。会在连接断开后尝试重连。
func Start() {
	if config.Conf == nil || config.Conf.CenterConfig == nil || config.Conf.CenterConfig.Address == "" {
		zap.L().Warn("center config address empty, skipping center ws start")
		return
	}

	// create and store package-level context so other background
	// goroutines (e.g. load reporter) can observe cancellation.
	ctx, c := context.WithCancel(context.Background())
	cancel = c
	ctxGlobal = ctx

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

			err := connectAndServe(ctxGlobal, config.Conf.CenterConfig.Address)
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
	// try ping center server first
	zap.L().Info("dialing center websocket", zap.String("addr", addr))
	d := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	c, _, err := d.Dial(addr, nil)
	if err != nil {
		return err
	}
	conn = c

	// mark connected and notify waiters
	readyMu.Lock()
	connected = true
	if readyCh != nil {
		close(readyCh)
		readyCh = nil
	}
	readyMu.Unlock()
	defer func() {
		_ = conn.Close()
		conn = nil

		// mark disconnected and recreate readyCh for future waits
		readyMu.Lock()
		connected = false
		if readyCh == nil {
			readyCh = make(chan struct{})
		}
		readyMu.Unlock()
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
		zap.L().Debug("center ws read message", zap.Int("msg_type", mt), zap.Int("data_len", len(data)))
		if err != nil {
			// If peer closed the connection normally (CloseNormalClosure / CloseGoingAway)
			// treat as closed and return so the reconnect loop can run.
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				zap.L().Info("center ws closed by peer", zap.Error(err))
				return err
			}

			// If it's a temporary network error (timeout/temporary), don't tear down the
			// whole connection immediately. Log and continue to allow transient blips.
			if ne, ok := err.(net.Error); ok && (ne.Timeout() || ne.Temporary()) {
				zap.L().Warn("temporary read error from center ws, will retry read", zap.Error(err))
				// small backoff to avoid busy-looping on persistent transient errors
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// EOF means remote closed; return to trigger reconnect
			if errors.Is(err, io.EOF) {
				zap.L().Info("center ws read EOF, connection closed", zap.Error(err))
				return err
			}

			// Other errors: treat as fatal for this connection and let reconnect loop handle it.
			zap.L().Warn("center ws read error, closing connection", zap.Error(err))
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
	zap.L().Debug("received message from center", zap.ByteString("data", data))

	var pushmsg types.PushMessage
	if err := json.Unmarshal(data, &pushmsg); err != nil {
		zap.L().Warn("handleIncoming: invalid json from center", zap.Error(err))
		return
	}
	zap.L().Debug("handleIncoming: parsed push message", zap.Any("message", pushmsg))
	err := push.Dispatch(pushmsg)
	if err != nil {
		zap.L().Error("handleIncoming: push.Dispatch error", zap.Error(err), zap.Any("message", pushmsg))
	}

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

// SendJSON 是对 sendJSONSafe 的导出封装，允许外部包通过 websocket 发送任意 JSON 可序列化结构。
func SendJSON(v interface{}) error {
	return sendJSONSafe(v)
}

// WaitReady waits until the websocket connection is established or the timeout elapses.
// It uses a mutex+channel notification mechanism to avoid polling.
func WaitReady(timeout time.Duration) error {
	// Fast path: check under lock if already connected
	readyMu.Lock()
	if connected {
		readyMu.Unlock()
		return nil
	}
	ch := readyCh
	readyMu.Unlock()

	if ch == nil {
		// connection already became ready between locks
		return nil
	}

	select {
	case <-ch:
		return nil
	case <-time.After(timeout):
		return errors.New("center ws not ready (timeout)")
	}
}

// ReportLoad sends a load update message to the center server.
// The message payload is: {"type":"load_update","load": <ratio 0..1>}
func ReportLoad(nowConn int, maxConn int) error {
	if maxConn <= 0 {
		return errors.New("invalid maxConn")
	}
	load := float64(nowConn) / float64(maxConn)
	msg := map[string]interface{}{
		"type": "load_update",
		"load": load,
		"ts":   time.Now().Unix(),
	}
	return sendJSONSafe(msg)
}

// StartLoadReporter starts a background goroutine that periodically reports
// the current connection load (now_conn / max_conn) to the center server.
// It will stop when the websocket Start/Stop context is canceled.
func StartLoadReporter(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// get current connection count from push manager
				now := push.GetConnectionCount()
				max := push.MaxConnections
				if max <= 0 {
					continue
				}
				if err := ReportLoad(now, max); err != nil {
					zap.L().Warn("load reporter: failed to send load", zap.Error(err))
				}
			case <-ctxGlobal.Done():
				zap.L().Info("load reporter exiting due to context done")
				return
			}
		}
	}()
}
