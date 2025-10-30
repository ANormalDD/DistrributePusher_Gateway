package config

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var Conf = new(AppConfig)

type AppConfig struct {
	Port      int    `mapstructure:"port"`
	Name      string `mapstructure:"name"`
	Mode      string `mapstructure:"mode"`
	Version   string `mapstructure:"version"`
	StartTime string `mapstructure:"start_time"`
	MachineID int64  `mapstructure:"machine_id"`

	*LogConfig        `mapstructure:"log"`
	*MySQLConfig      `mapstructure:"mysql"`
	*RedisConfig      `mapstructure:"redis"`
	*JWTConfig        `mapstructure:"jwt"`
	*DispatcherConfig `mapstructure:"dispatcher"`
	*CenterConfig     `mapstructure:"center"`
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"dbname"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}
type JWTConfig struct {
	Secret         string `mapstructure:"secret"`
	ExpireDuration int    `mapstructure:"expire_duration"`
}

type DispatcherConfig struct {
	WorkerCount    int `mapstructure:"worker_count"`
	TaskQueueSize  int `mapstructure:"task_queue_size"`
	MaxConnections int `mapstructure:"max_connections"`
}

type CenterConfig struct {
	Address string `mapstructure:"address"`
}

func Init() (err error) {
	viper.SetConfigName("config") // 指定配置文件名称（不带后缀）
	viper.SetConfigType("yaml")   // 指定配置文件类型
	viper.AddConfigPath(".")      // 指定查找配置文件的路径（这里使用相对路径）
	err = viper.ReadInConfig()    // 读取配置信息
	if err != nil {               // 读取配置信息失败
		fmt.Printf("viper.ReadInConfig() failed, err:%v\n", err)
		return
	}
	if err = viper.Unmarshal(Conf); err != nil { // 将读取到的配置信息反序列化到Conf变量中
		fmt.Printf("viper.Unmarshal failed, err:%v\n", err)
	}
	// 监听配置文件变化
	viper.WatchConfig()
	viper.OnConfigChange(func(in fsnotify.Event) {
		fmt.Println("配置文件修改了...")
		if err = viper.Unmarshal(Conf); err != nil {
			fmt.Printf("viper.Unmarshal failed, err:%v\n", err)
		}
	})
	return
}
