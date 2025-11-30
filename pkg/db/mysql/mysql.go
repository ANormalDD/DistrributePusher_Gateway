package mysql

import (
	"Gateway/pkg/config"
	"Gateway/pkg/monitor"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var DB *sqlx.DB
var Monitor *monitor.Monitor

func Init(cfg *config.MySQLConfig) (err error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)
	fmt.Println(dsn)

	// instrument connection attempt
	Monitor = monitor.NewMonitor("mysql_connect", 100, 10000, 60000)
	t := monitor.NewTask()
	DB, err = sqlx.Connect("mysql", dsn)
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
