package push

import (
	"Gateway/center_client"
	"Gateway/pkg/config"
	"Gateway/pkg/db/redis"
	"Gateway/pkg/push/types"
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go.uber.org/zap"
)


var dispatcherCtx context.Context
var dispatcherCancel context.CancelFunc
var MaxConnections int = 100000 // default max connections

func Dispatch(msg types.PushMessage) error {

	clientMsg := types.ClientMessage{
		ID:       msg.ID,
		Type:     msg.Type,
		RoomID:   msg.RoomID,
		SenderID: msg.SenderID,
		Payload:  msg.Payload,
	}

	marshaledMsg, err := json.Marshal(clientMsg)
	if err != nil {
		zap.L().Error("Failed to marshal client message", zap.Error(err), zap.Any("message", clientMsg))
		return err
	}
	for _, uid := range msg.TargetIDs {
		// try to enqueue directly to the user's send channel; small timeout to avoid blocking
		if err := EnqueueMessage(uid, 100*time.Millisecond, clientMsg); err != nil {
			if err == ErrNoConn {
				zap.L().Error("User not connected,try to push back msg", zap.Int64("userID", uid), zap.Error(err))
				// send push back request to center server
				forwardReq := types.ClientMessage{
					ID:       msg.ID,
					Type:     msg.Type,
					RoomID:   msg.RoomID,
					SenderID: msg.SenderID,
					Payload:  msg.Payload,
				}
				err2 := center_client.SendPushBackRequest(config.Conf.CenterConfig, forwardReq)
				if err2 != nil {
					zap.L().Error("SendPushBackRequest failed", zap.Int64("userID", uid), zap.Error(err2))
				}
				continue
			}
			// fallback: 推送到redis等待队列,并记录
			err2 := redis.SAddWithRetry(2, "wait:push:set", strconv.FormatInt(uid, 10))
			if err2 != nil {
				zap.L().Error("EnqueueMessage failed and SAdd wait push userID failed. Message missed", zap.Int64("userID", uid), zap.Error(err2), zap.Error(err))
				continue
			}
			err2 = redis.RPushWithRetry(2, "wait:push:"+strconv.FormatInt(uid, 10), marshaledMsg)
			if err2 != nil {
				zap.L().Error("EnqueueMessage failed and RPush wait push message failed. Message missed", zap.Int64("userID", uid), zap.Error(err2), zap.Error(err))
			}
		}
	}
	return nil
}

func waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zap.L().Info("Push dispatcher shutdown signal received, canceling dispatcher context")
	if dispatcherCancel != nil {
		dispatcherCancel()
	}
	// ListeningWaitQueue and other goroutines should observe dispatcherCtx.Done() and exit.
	zap.L().Info("Push dispatcher shutdown initiated; background listeners will exit")
}

func InitDispatcher(Conf *config.DispatcherConfig) {
	dispatcherCtx, dispatcherCancel = context.WithCancel(context.Background())
	MaxConnections = Conf.MaxConnections
	go waitForShutdown()
	go ListeningWaitQueue()
}
