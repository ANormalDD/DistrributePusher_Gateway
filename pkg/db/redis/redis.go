package redis

import (
	"Gateway/pkg/config"
	"Gateway/pkg/monitor"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var Rdb *redis.Client
var Monitor *monitor.Monitor

func Init(cfg *config.RedisConfig) (err error) {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password, // no password set
		DB:       cfg.DB,       // use default DB
		PoolSize: cfg.PoolSize,
	})
	Monitor = monitor.NewMonitor("redis", 1000, 10000, 60000)
	// register hook so all redis commands are monitored centrally
	Rdb.AddHook(&redisMonitorHook{mon: Monitor})
	ctx := context.Background()
	_, err = Rdb.Ping(ctx).Result()
	return
}

func LPopWithRetry(retry int, key string) (string, error) {
	var err error
	var result string
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, err = Rdb.LPop(ctx, key).Result()
		if err == nil {
			return result, nil
		}
		if err == redis.Nil {
			return "", redis.Nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}
func RPushWithRetry(retry int, key string, value interface{}) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.RPush(ctx, key, value).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// set operation
func SAddWithRetry(retry int, key string, members ...interface{}) error {
	var err error
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		err = Rdb.SAdd(ctx, key, members...).Err()
		if err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// query everything in set
func SMembersWithRetry(retry int, key string) ([]string, error) {
	var err error
	var result []string
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, err = Rdb.SMembers(ctx, key).Result()
		if err == nil {
			return result, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}
func SScanWithRetry(retry int, key string, cursor uint64, match string, count int64) ([]string, uint64, error) {
	var err error
	var result []string
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		result, cursor, err = Rdb.SScan(ctx, key, cursor, match, count).Result()
		if err == nil {
			return result, cursor, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return result, cursor, err
}
func SIsEmptyWithRetry(retry int, key string) (bool, error) {
	var err error
	var count int64
	for i := 0; i < retry; i++ {
		ctx := context.Background()
		count, err = Rdb.SCard(ctx, key).Result()
		if err == nil {
			return count == 0, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false, err
}

func Close() {
	_ = Rdb.Close()
}
