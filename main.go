package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"Gateway/center_client"
	cws "Gateway/center_client/ws"
	"Gateway/pkg/config"
	"Gateway/pkg/db/mysql"
	"Gateway/pkg/db/redis"
	"Gateway/pkg/logger"
	"Gateway/pkg/push"
	"Gateway/route"

	"go.uber.org/zap"
)

func main() {
	// initialize config
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to init config: %v\n", err)
		os.Exit(1)
	}

	// init logger
	if config.Conf.LogConfig != nil {
		if err := logger.Init(config.Conf.LogConfig); err != nil {
			fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
			os.Exit(1)
		}
	}

	zap.L().Info("config and logger initialized")

	// init mysql (optional)
	if config.Conf.MySQLConfig != nil {
		if err := mysql.Init(config.Conf.MySQLConfig); err != nil {
			zap.L().Warn("mysql init failed, continuing without mysql", zap.Error(err))
		} else {
			defer mysql.Close()
		}
	}

	// init redis (optional)
	if config.Conf.RedisConfig != nil {
		if err := redis.Init(config.Conf.RedisConfig); err != nil {
			zap.L().Warn("redis init failed, continuing without redis", zap.Error(err))
		} else {
			defer redis.Close()
		}
	}

	// init push dispatcher
	if config.Conf.DispatcherConfig != nil {
		push.InitDispatcher(config.Conf.DispatcherConfig)
	}

	// register to center in background (non-blocking)
	gatewayAddr := config.Conf.Address
	gatewayPort := config.Conf.Port
	maxConns := 0
	if config.Conf.DispatcherConfig != nil {
		maxConns = config.Conf.DispatcherConfig.MaxConnections
	}
	go func() {
		if config.Conf.CenterConfig == nil {
			zap.L().Warn("center config nil, skipping register to center")
			return
		}
		if err := center_client.RegisterToCenter(config.Conf.CenterConfig, gatewayAddr, gatewayPort, maxConns); err != nil {
			zap.L().Warn("register to center failed", zap.Error(err))
		} else {
			zap.L().Info("registered to center successfully")
		}
	}()

	// start HTTP server (gin)
	r := route.InitRouter()
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", config.Conf.Port),
		Handler: r,
	}

	go func() {
		zap.L().Info("starting http server", zap.Int("port", config.Conf.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Fatal("http server error", zap.Error(err))
		}
	}()

	// wait for termination signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zap.L().Info("shutting down server...")

	// graceful shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		zap.L().Error("server shutdown error", zap.Error(err))
	}

	// stop center ws
	if err := cws.Stop(); err != nil {
		zap.L().Warn("failed to stop center ws", zap.Error(err))
	}

	zap.L().Info("server exited")
}
