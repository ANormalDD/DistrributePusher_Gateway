package push

import (
	"Gateway/center"
	"Gateway/pkg/config"
	"Gateway/pkg/db/redis"
	"Gateway/pkg/push/types"
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

var PushTaskChan chan types.PushTask = make(chan types.PushTask, 1000)

var dispatcherCtx context.Context
var dispatcherCancel context.CancelFunc
var dispatcherWg sync.WaitGroup

func startPushDispatcher(Conf *config.DispatcherConfig) {
	PushTaskChan = make(chan types.PushTask, Conf.TaskQueueSize)
	for i := 0; i < Conf.WorkerCount; i++ {
		dispatcherWg.Add(1)
		go func() {
			defer dispatcherWg.Done()
			for {
				select {
				case <-dispatcherCtx.Done():
					return
				case task, ok := <-PushTaskChan:
					if !ok {
						return
					}
					// protect each task processing so a panic doesn't kill the worker
					func() {
						defer func() {
							if r := recover(); r != nil {
								zap.L().Error("Panic recovered in push worker", zap.Any("recover", r))
							}
						}()
						err := PushViaWSWithRetry(task.UserID, 2, 10*time.Second, task.Msg)
						if err != nil {
							// If WebSocket push fails, push to center for forwarding
							err2 := center.SendForwardRequestWithRetry(config.Conf.CenterConfig, task.Msg, 3, 100)
							if err2 != nil {
								zap.L().Error("Failed to forward message to center after WS push failure, msg missed", zap.Int64("userID", task.UserID), zap.Error(err2))
							}
						}
					}()
				}
			}
		}()
	}
}

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
		select {
		case PushTaskChan <- types.PushTask{
			UserID:       uid,
			Msg:          clientMsg,
			MarshaledMsg: marshaledMsg,
		}:
		default:
			{
				// 推送到redis等待队列,并记录
				err2 := redis.SAddWithRetry(2, "wait:push:set", strconv.FormatInt(uid, 10))
				if err2 != nil {
					zap.L().Error("PushTaskChan full, failed to SAdd wait push userID. Message missed", zap.Int64("userID", uid), zap.Error(err2))
					continue
				}
				err2 = redis.RPushWithRetry(2, "wait:push:"+strconv.FormatInt(uid, 10), marshaledMsg)
				if err2 != nil {
					zap.L().Error("PushTaskChan full, failed to RPush wait push message. Message missed", zap.Int64("userID", uid), zap.Error(err2))
				}
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
	// drain buffered tasks in PushTaskChan back to Redis wait queues to avoid message loss
	zap.L().Info("Draining PushTaskChan to Redis")
	for {
		select {
		case task := <-PushTaskChan:
			// write back to wait queue per user
			uidStr := strconv.FormatInt(task.UserID, 10)
			if err := redis.SAddWithRetry(2, "wait:push:set", uidStr); err != nil {
				zap.L().Error("Failed to SAdd wait push userID during shutdown", zap.Int64("userID", task.UserID), zap.Error(err))
			}
			if err := redis.RPushWithRetry(2, "wait:push:"+uidStr, task.MarshaledMsg); err != nil {
				zap.L().Error("Failed to RPush wait push message during shutdown. Message missed", zap.Int64("userID", task.UserID), zap.Error(err))
			}
		default:
			// channel drained
			goto afterDrain
		}
	}
afterDrain:
	// wait for workers to exit
	dispatcherWg.Wait()
	zap.L().Info("Push dispatcher workers exited")
}

func InitDispatcher(Conf *config.DispatcherConfig) {
	dispatcherCtx, dispatcherCancel = context.WithCancel(context.Background())
	MaxConnections = Conf.MaxConnections
	startPushDispatcher(Conf)
	go waitForShutdown()
	go ListeningWaitQueue()
}
