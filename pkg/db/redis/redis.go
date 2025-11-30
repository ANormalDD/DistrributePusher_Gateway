package redis

import (
	"Gateway/pkg/config"
	"Gateway/pkg/monitor"
	"fmt"
	"time"

	"github.com/go-redis/redis"
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
	_, err = Rdb.Ping().Result()
	return
}

func LPopWithRetry(retry int, key string) (string, error) {
	var err error
	var result string
	for i := 0; i < retry; i++ {
		task := monitor.NewTask()
		result, err = Rdb.LPop(key).Result()
		if err == nil {
			Monitor.CompleteTask(task, true)
			return result, nil
		}
		if err == redis.Nil {
			Monitor.CompleteTask(task, false)
			return "", redis.Nil
		}
		Monitor.CompleteTask(task, false)
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}
func RPushWithRetry(retry int, key string, value interface{}) error {
	var err error
	for i := 0; i < retry; i++ {
		task := monitor.NewTask()
		err = Rdb.RPush(key, value).Err()
		if err == nil {
			Monitor.CompleteTask(task, true)
			return nil
		}
		Monitor.CompleteTask(task, false)
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// set operation
func SAddWithRetry(retry int, key string, members ...interface{}) error {
	var err error
	for i := 0; i < retry; i++ {
		task := monitor.NewTask()
		err = Rdb.SAdd(key, members...).Err()
		if err == nil {
			Monitor.CompleteTask(task, true)
			return nil
		}
		Monitor.CompleteTask(task, false)
		time.Sleep(100 * time.Millisecond)
	}
	return err
}

// query everything in set
func SMembersWithRetry(retry int, key string) ([]string, error) {
	var err error
	var result []string
	for i := 0; i < retry; i++ {
		task := monitor.NewTask()
		result, err = Rdb.SMembers(key).Result()
		if err == nil {
			Monitor.CompleteTask(task, true)
			return result, nil
		}
		Monitor.CompleteTask(task, false)
		time.Sleep(100 * time.Millisecond)
	}
	return result, err
}
func SScanWithRetry(retry int, key string, cursor uint64, match string, count int64) ([]string, uint64, error) {
	var err error
	var result []string
	for i := 0; i < retry; i++ {
		task := monitor.NewTask()
		result, cursor, err = Rdb.SScan(key, cursor, match, count).Result()
		if err == nil {
			Monitor.CompleteTask(task, true)
			return result, cursor, nil
		}
		Monitor.CompleteTask(task, false)
		time.Sleep(100 * time.Millisecond)
	}
	return result, cursor, err
}
func SIsEmptyWithRetry(retry int, key string) (bool, error) {
	var err error
	var count int64
	for i := 0; i < retry; i++ {
		task := monitor.NewTask()
		count, err = Rdb.SCard(key).Result()
		if err == nil {
			Monitor.CompleteTask(task, true)
			return count == 0, nil
		}
		Monitor.CompleteTask(task, false) 
		time.Sleep(100 * time.Millisecond)
	}
	return false, err
}

func Close() {
	_ = Rdb.Close()
}
