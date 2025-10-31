package push

import (
	"Gateway/center_client/ws"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 单一存储，value: *ConnectionHolder
var connStore sync.Map // key: int64, value: *ConnectionHolder

var ErrNoConn = errors.New("no connection for user")
var ConnCount = 0
var ConnCountMu sync.RWMutex
var MaxConnections = 10000 // example limit, can be set from config

type ConnectionHolder struct {
	Conn *websocket.Conn
	Mu   sync.Mutex
}

func GetConnectionCount() int {
	ConnCountMu.RLock()
	defer ConnCountMu.RUnlock()
	return ConnCount
}
func IncrementConnectionCount() {
	ConnCountMu.Lock()
	ConnCount++
	ConnCountMu.Unlock()
}

func DecrementConnectionCount() {
	ConnCountMu.Lock()
	if ConnCount > 0 {
		ConnCount--
	}
	ConnCountMu.Unlock()
}

// WriteJSONSafe 写操作封装在 holder 内，确保在持锁后检查 conn 并设置写超时
func WriteJSONSafe(holder *ConnectionHolder, timeout time.Duration, message interface{}) error {
	holder.Mu.Lock()
	defer holder.Mu.Unlock()
	if holder.Conn == nil {
		return ErrNoConn
	}
	holder.Conn.SetWriteDeadline(time.Now().Add(timeout))
	return holder.Conn.WriteJSON(message)
}

// GetConnectionHolder 返回 holder，用于在调用方持锁/访问 Conn
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
		holder.Mu.Lock()
		if holder.Conn != nil {
			holder.Conn.Close()
		}
		connStore.Delete(userID)
		holder.Conn = nil
		holder.Mu.Unlock()
		DecrementConnectionCount()
	}
	holder := &ConnectionHolder{Conn: conn}
	connStore.Store(userID, holder)
	ws.ReportUserConnect(userID)
	IncrementConnectionCount()
}

func RemoveConnection(userID int64) error {
	val, ok := connStore.Load(userID)
	if !ok {
		return ErrNoConn
	}
	holder := val.(*ConnectionHolder)
	holder.Mu.Lock()
	if holder.Conn != nil {
		holder.Conn.Close()
	}
	connStore.Delete(userID)
	holder.Conn = nil
	holder.Mu.Unlock()
	ws.ReportUserDisconnect(userID)
	DecrementConnectionCount()
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

func GetConnectionLock(userID int64) (*sync.Mutex, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, false
	}
	holder := val.(*ConnectionHolder)
	return &holder.Mu, true
}

func GetConnectionAndLock(userID int64) (*websocket.Conn, *sync.Mutex, bool) {
	val, ok := connStore.Load(userID)
	if !ok {
		return nil, nil, false
	}
	holder := val.(*ConnectionHolder)
	return holder.Conn, &holder.Mu, true
}
