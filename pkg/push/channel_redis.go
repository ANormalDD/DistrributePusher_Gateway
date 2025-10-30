package push

import (
	"Gateway/pkg/db/redis"
	"Gateway/pkg/push/types"
	"encoding/json"
	"strconv"
	"time"

	Redis "github.com/go-redis/redis"
	"go.uber.org/zap"
)

// 登录时将存在redis中的离线消息推送给用户，然后删除redis中的离线消息 (作为Gateway不需要此功能)
func PushOfflineMessages(userID int64) {
	lenOfList, err := redis.Rdb.LLen("offline:push:" + strconv.FormatInt(userID, 10)).Result()
	if err != nil {
		if err != Redis.Nil {
			zap.L().Error("Failed to get length of offline message list", zap.Int64("userID", userID), zap.Error(err))
			//retry once
			time.Sleep(100 * time.Millisecond)
			lenOfList, err = redis.Rdb.LLen("offline:push:" + strconv.FormatInt(userID, 10)).Result()
			if err != nil {
				zap.L().Error("Failed to get length of offline message list again, giving up", zap.Int64("userID", userID), zap.Error(err))
				return
			}
		} else {
			return
		}
	}
	var messages []types.ClientMessage
	var msg string
	var batchSize int = 50
	var nowSize int = 0
	for i := int64(0); i < lenOfList; i++ {
		if nowSize >= batchSize {
			// push current batch
			err = PushViaWSWithRetry(userID, 2, 10*time.Second, types.ClientMessage{
				ID:      -1,
				Type:    "batch",
				Payload: messages,
			})
			if err != nil {
				raw, _ := json.Marshal(messages)
				err2 := redis.RPushWithRetry(2, "offline:push:"+strconv.FormatInt(userID, 10), raw)
				if err2 != nil {
					zap.L().Error("Failed to RPush offline message back .Message missed", zap.Int64("userID", userID), zap.Error(err2))
				}
				if err != ErrNoConn {
					RemoveConnection(userID)
				}
			}
			// reset batch
			messages = []types.ClientMessage{}
			nowSize = 0
		}
		msg, err = redis.LPopWithRetry(2, "offline:push:"+strconv.FormatInt(userID, 10))
		if err != nil {
			if err == Redis.Nil {
				// No more messages
				break
			}
			// Log the error and break to avoid infinite loop
			zap.L().Error("Failed to LPop offline message", zap.Int64("userID", userID), zap.Error(err))
			//info client that there are offline messages but failed to get them
			err = PushViaWSWithRetry(userID, 2, 10*time.Second, types.ClientMessage{
				ID:      -1,
				Type:    "info",
				Payload: "You have offline messages but failed to retrieve them.",
			})

			if err != nil && err != ErrNoConn {
				zap.L().Error("Failed to push offline message info", zap.Int64("userID", userID), zap.Error(err))
				RemoveConnection(userID)
			}

			break
		}
		var clientMsg types.ClientMessage
		if err := json.Unmarshal([]byte(msg), &clientMsg); err != nil {
			continue
		}
		messages = append(messages, clientMsg)
	}
	if len(messages) > 0 {
		err = PushViaWSWithRetry(userID, 2, 10*time.Second, types.ClientMessage{
			ID:      -1,
			Type:    "batch",
			Payload: messages,
		})
		if err != nil {
			raw, _ := json.Marshal(messages)
			err2 := redis.RPushWithRetry(2, "offline:push:"+strconv.FormatInt(userID, 10), raw)
			if err2 != nil {
				zap.L().Error("Failed to RPush offline message back .Message missed", zap.Int64("userID", userID), zap.Error(err2))
			}
		}
	}
}

func ListeningWaitQueue() {
	// 轮询等待队列，如果有消息且 PushTaskChan 负载在 80% 以下则取出消息进行推送
	for {
		// exit promptly if dispatcher is shutting down
		select {
		case <-dispatcherCtx.Done():
			zap.L().Info("ListeningWaitQueue exiting due to dispatcher shutdown")
			return
		default:
		}

		// Use SSCAN to iterate the set in pages to avoid fetching all members at once
		var cursor uint64 = 0
		var pageSize int64 = 100
		for {
			members, nextCursor, err := redis.SScanWithRetry(2, "wait:push:set", cursor, "", pageSize)
			if err != nil {
				zap.L().Error("Failed to SScan wait push set", zap.Error(err))
				break
			}
			for _, uidStr := range members {
				// check shutdown signal inside the inner loop as well
				select {
				case <-dispatcherCtx.Done():
					zap.L().Info("ListeningWaitQueue exiting due to dispatcher shutdown")
					return
				default:
				}
				uid, err := strconv.ParseInt(uidStr, 10, 64)
				if err != nil {
					continue
				}
				queueLen := len(PushTaskChan)
				if queueLen < cap(PushTaskChan)*8/10 {
					// pop message from wait queue
					msgStr, err := redis.LPopWithRetry(2, "wait:push:"+uidStr)
					if err != nil {
						if err != Redis.Nil {
							zap.L().Error("Failed to LPop wait push message", zap.Int64("userID", uid), zap.Error(err))
						}
						continue
					}
					var clientMsg types.ClientMessage
					if err := json.Unmarshal([]byte(msgStr), &clientMsg); err != nil {
						continue
					}
					// send to PushTaskChan
					select {
					case PushTaskChan <- types.PushTask{
						UserID:       uid,
						Msg:          clientMsg,
						MarshaledMsg: []byte(msgStr),
					}:
					default:
						// push back to wait queue
						err2 := redis.RPushWithRetry(2, "wait:push:"+uidStr, msgStr)
						if err2 != nil {
							zap.L().Error("Failed to RPush wait push message back. Message missed", zap.Int64("userID", uid), zap.Error(err2))
						}
						goto QUEUEBUSY
					}
					// if wait queue is empty remove from set
					lenOfWaitQueue, err := redis.Rdb.LLen("wait:push:" + uidStr).Result()
					if err != nil {
						if err != Redis.Nil {
							zap.L().Error("Failed to get length of wait push queue", zap.Int64("userID", uid), zap.Error(err))
							continue
						}
						continue
					}
					if lenOfWaitQueue == 0 {
						err = redis.Rdb.SRem("wait:push:set", uidStr).Err()
						if err != nil {
							zap.L().Error("Failed to remove userID from wait push set", zap.Int64("userID", uid), zap.Error(err))
						}
					}
				}
			}
		QUEUEBUSY:
			if nextCursor == 0 {
				break
			}
			cursor = nextCursor
			time.Sleep(1000 * time.Millisecond)
		}

	}
}
