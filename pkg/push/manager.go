package push

import (
	"errors"
	"sync"
	"time"

	"Gateway/pkg/config"

	"github.com/gorilla/websocket"
)

// 单一存储，value: *ConnectionHolder
var connStore sync.Map // key: int64, value: *ConnectionHolder

var ErrNoConn = errors.New("no connection for user")

var connCount int = 0
var connCountLock sync.Mutex

type sendRequest struct {
	msg  interface{}
	resp chan error
}

type ConnectionHolder struct {
	Conn    *websocket.Conn
	sendCh  chan sendRequest
	closeCh chan struct{}
}

// writerLoop 串行化对 websocket 的写入并在完成后将结果返回给请求方
func writerLoop(ch *ConnectionHolder) {
	for {
		select {
		case req, ok := <-ch.sendCh:
			if !ok {
				return
			}
			var err error
			// If caller passed a websocket control message type (e.g. websocket.PingMessage)
			// write it as a control frame instead of JSON.
			if mt, ok := req.msg.(int); ok && mt == websocket.PingMessage {
				err = ch.Conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
			} else {
				ch.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err = ch.Conn.WriteJSON(req.msg)
			}
			// send result back if caller expects it
			if req.resp != nil {
				select {
				case req.resp <- err:
				default:
				}
			}
			if err != nil {
				// on write error, close connection and exit writer
				ch.Conn.Close()
				return
			}
		case <-ch.closeCh:
			return
		}
	}
}

// WriteJSONSafe 将消息封装成请求并发送到连接的 writer goroutine，等待写入完成或超时
func WriteJSONSafe(holder *ConnectionHolder, timeout time.Duration, message interface{}) error {
	if holder == nil {
		return ErrNoConn
	}
	// prepare request
	req := sendRequest{msg: message, resp: make(chan error, 1)}
	// enqueue with timeout
	select {
	case holder.sendCh <- req:
		// wait for write result or timeout
		select {
		case err := <-req.resp:
			if err != nil {
				return err
			}
			return nil
		case <-time.After(timeout):
			return errors.New("write result timeout")
		}
	case <-time.After(timeout):
		return errors.New("enqueue timeout")
	}
}

// EnqueueMessage 将消息尽力推入连接发送队列，不等待写入执行，仅等待入队成功或超时。
func EnqueueMessage(userID int64, timeout time.Duration, message interface{}) error {
	val, ok := connStore.Load(userID)
	if !ok {
		return ErrNoConn
	}
	holder := val.(*ConnectionHolder)
	req := sendRequest{msg: message, resp: nil}
	select {
	case holder.sendCh <- req:
		return nil
	case <-time.After(timeout):
		return errors.New("enqueue timeout")
	}
}

// GetConnectionHolder 返回 holder，用于在调用方检查/访问 Conn
func GetConnectionHolder(userID int64) (*ConnectionHolder, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, false
	}
	return val.(*ConnectionHolder), true
}

func RegisterConnection(userID int64, conn *websocket.Conn) {
	// 如果已有旧连接，先关闭并删除
	if val, ok := connStore.Load(userID); ok {
		holder := val.(*ConnectionHolder)
		// signal close
		func() {
			defer func() { _ = recover() }()
			close(holder.closeCh)
			close(holder.sendCh)
		}()
		if holder.Conn != nil {
			holder.Conn.Close()
		}
		connCountLock.Lock()
		connCount--
		connCountLock.Unlock()
		connStore.Delete(userID)
	}
	// determine send channel buffer size from config, fallback to 128
	buf := 128
	if config.Conf != nil && config.Conf.DispatcherConfig != nil && config.Conf.DispatcherConfig.SendChannelSize > 0 {
		buf = config.Conf.DispatcherConfig.SendChannelSize
	}
	holder := &ConnectionHolder{
		Conn:    conn,
		sendCh:  make(chan sendRequest, buf),
		closeCh: make(chan struct{}),
	}
	connStore.Store(userID, holder)
	connCountLock.Lock()
	connCount++
	connCountLock.Unlock()
	go writerLoop(holder)
}

func RemoveConnection(userID int64) error {
	val, ok := connStore.Load(userID)
	if !ok {
		return ErrNoConn
	}
	holder := val.(*ConnectionHolder)
	// try close channels safely
	func() {
		defer func() { _ = recover() }()
		close(holder.closeCh)
		close(holder.sendCh)
	}()
	if holder.Conn != nil {
		holder.Conn.Close()
	}
	connStore.Delete(userID)
	connCountLock.Lock()
	connCount--
	connCountLock.Unlock()
	return nil
}

func GetConnection(userID int64) (*websocket.Conn, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, false
	}
	holder := val.(*ConnectionHolder)
	return holder.Conn, true
}

// compatibility: keep these helpers but note they are no-ops for lock-based access
func GetConnectionLock(userID int64) (*sync.Mutex, bool) {
	return nil, false
}

func GetConnectionAndLock(userID int64) (*websocket.Conn, *sync.Mutex, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, nil, false
	}
	holder := val.(*ConnectionHolder)
	return holder.Conn, nil, true
}

func GetConnectionCount() int {
	connCountLock.Lock()
	defer connCountLock.Unlock()
	return connCount
}
