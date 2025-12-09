package push

import (
	"Gateway/pkg/db/redis"
	"Gateway/pkg/push/types"
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	Redis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var waitSet sync.Map

// use when redis is down <int,chan<string>>
var waitMsg sync.Map
var offlineMsg sync.Map

func InsertWaitSet(userID int64) {
	_, ok := waitSet.Load(userID)
	if ok {
		return
	}
	waitSet.Store(userID, struct{}{})
}

func RemoveWaitSet(userID int64) {
	_, ok := waitSet.Load(userID)
	if !ok {
		return
	}
	waitSet.Delete(userID)
}

func InsertWaitMsg(userID int64, msg string) {
	chInterface, ok := waitMsg.Load(userID)
	if !ok {
		ch := make(chan string, 100)
		waitMsg.Store(userID, ch)
		chInterface = ch
	}
	ch := chInterface.(chan string)
	ch <- msg
}

func InsertWaitQueue(userID int64, marshaledMsg string) {
	InsertWaitSet(userID)
	err := redis.RPushWithRetry(2, "wait:push:"+strconv.FormatInt(userID, 10), marshaledMsg)
	if err != nil {
		zap.L().Error("Failed to RPush wait push message,try store local", zap.Int64("userID", userID), zap.Error(err))
		InsertWaitMsg(userID, marshaledMsg)
	}
}

func ListeningWaitQueue() {
	for {
		// exit promptly if dispatcher is shutting down
		select {
		case <-dispatcherCtx.Done():
			zap.L().Info("ListeningWaitQueue exiting due to dispatcher shutdown")
			return
		default:
		}

		// Iterate local waitSet instead of scanning Redis set.
		anyMember := false
		waitSet.Range(func(k, _ interface{}) bool {
			// check shutdown signal inside the inner loop as well
			select {
			case <-dispatcherCtx.Done():
				zap.L().Info("ListeningWaitQueue exiting due to dispatcher shutdown")
				return false
			default:
			}

			var uid int64
			switch v := k.(type) {
			case int64:
				uid = v
			case int:
				uid = int64(v)
			case string:
				parsed, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return true
				}
				uid = parsed
			default:
				return true
			}

			anyMember = true
			uidStr := strconv.FormatInt(uid, 10)

			// pop message from wait queue
			msgStr, err := redis.LPopWithRetry(2, "wait:push:"+uidStr)
			if err != nil {
				if err != Redis.Nil {
					zap.L().Error("Failed to LPop wait push message", zap.Int64("userID", uid), zap.Error(err))
				}
				return true
			}
			var clientMsg types.ClientMessage
			if err := json.Unmarshal([]byte(msgStr), &clientMsg); err != nil {
				// malformed message, skip
				return true
			}
			// try enqueue directly to the user's send channel
			if err := EnqueueMessage(uid, 100*time.Millisecond, clientMsg); err != nil {
				// push back to wait queue
				err2 := redis.RPushWithRetry(2, "wait:push:"+uidStr, msgStr)
				if err2 != nil {
					zap.L().Error("Failed to RPush wait push message back. Message missed", zap.Int64("userID", uid), zap.Error(err2))
				}
				return false // stop iteration when queue is busy
			}
			// if wait queue is empty remove from local waitSet
			ctx := context.Background()
			lenOfWaitQueue, err := redis.Rdb.LLen(ctx, "wait:push:" + uidStr).Result()
			if err != nil {
				if err != Redis.Nil {
					zap.L().Error("Failed to get length of wait push queue", zap.Int64("userID", uid), zap.Error(err))
					return true
				}
				return true
			}
			if lenOfWaitQueue == 0 {
				RemoveWaitSet(uid)
			}
			return true
		})

		if !anyMember {
			time.Sleep(1000 * time.Millisecond)
			continue
		}

	}
}
