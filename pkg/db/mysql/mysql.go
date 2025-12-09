package mysql

import (
	"Gateway/pkg/config"
	"Gateway/pkg/monitor"
	"context"

	"database/sql"
	"fmt"

	"github.com/qustavo/sqlhooks/v2"

	mysql_with_hooks "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var DB *sqlx.DB
var Monitor *monitor.Monitor

type monitorHook struct {
	monitor *monitor.Monitor
}

func (h *monitorHook) Before(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	t := monitor.NewTask()
	ctx = context.WithValue(ctx, "monitor_task", t)
	return ctx, nil
}

func (h *monitorHook) After(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	t, ok := ctx.Value("monitor_task").(*monitor.Task)
	if ok && h.monitor != nil {
		h.monitor.CompleteTask(t, true)
	}
	return ctx, nil
}

func (h *monitorHook) OnError(ctx context.Context, err error, query string, args ...interface{}) error {
	t, ok := ctx.Value("monitor_task").(*monitor.Task)
	if ok && h.monitor != nil {
		h.monitor.CompleteTask(t, false)
	}
	return err
}

func Init(cfg *config.MySQLConfig) (err error) {
	Monitor = monitor.NewMonitor("mysql", 100, 10000, 60000)
	sql.Register("monitor_hook_mysql", sqlhooks.Wrap(&mysql_with_hooks.MySQLDriver{}, &monitorHook{monitor: Monitor}))
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)
	fmt.Println(dsn)

	// instrument connection attempt

	t := monitor.NewTask()
	DB, err = sqlx.Connect("monitor_hook_mysql", dsn)
	success := err == nil
	if Monitor != nil {
		Monitor.CompleteTask(t, success)
	}
	if err != nil {
		return err
	}
	DB.SetMaxOpenConns(cfg.MaxOpenConns)
	DB.SetMaxIdleConns(cfg.MaxIdleConns)
	return nil
}

func Close() {
	_ = DB.Close()
}
