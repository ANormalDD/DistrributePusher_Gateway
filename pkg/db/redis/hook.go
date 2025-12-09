package redis

import (
	"context"
	"net"

	"Gateway/pkg/monitor"

	"github.com/redis/go-redis/v9"
)

type redisMonitorHook struct {
	mon *monitor.Monitor
}

func (h *redisMonitorHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return next(ctx, network, addr)
	}
}

func (h *redisMonitorHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		task := monitor.NewTask()
		err := next(ctx, cmd)
		if err == nil {
			h.mon.CompleteTask(task, true)
		} else {
			h.mon.CompleteTask(task, false)
		}
		return err

	}
}

func (h *redisMonitorHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		task := monitor.NewTask()
		err := next(ctx, cmds)
		if err == nil {
			h.mon.CompleteTask(task, true)
		} else {
			h.mon.CompleteTask(task, false)
		}
		return err
	}
}
