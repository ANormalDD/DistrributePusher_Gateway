package center

import (
	"Gateway/center/ws"
	"Gateway/pkg/config"
	"Gateway/pkg/push/types"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type RegisterGatewayRequest struct {
	Address        string `json:"address"`
	Port           int    `json:"port"`
	MaxConnections int    `json:"max_connections"`
}

func RegisterToCenter(cfg *config.CenterConfig, gatewayAddress string, gatewayPort int, maxConnections int) error {
	ws.Start()
	reqBody := RegisterGatewayRequest{
		Address:        gatewayAddress,
		Port:           gatewayPort,
		MaxConnections: maxConnections,
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		zap.L().Error("Failed to marshal register gateway request", zap.Error(err))
		return err
	}
	resp, err := http.Post(cfg.Address+"/register_gateway", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		zap.L().Error("Failed to send register gateway request", zap.Error(err))
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		zap.L().Error("Failed to register gateway", zap.String("status", resp.Status))
		return fmt.Errorf("failed to register gateway: %s", resp.Status)
	}
	return nil
}

// 对没有连接到本gateway的用户转发消息到center，由center负责路由到正确的gateway center_address/forward
func SendForwardRequest(cfg *config.CenterConfig, forwardReq types.ClientMessage) error {
	jsonData, err := json.Marshal(forwardReq)
	if err != nil {
		zap.L().Error("Failed to marshal forward request", zap.Error(err))
		return err
	}
	resp, err := http.Post(cfg.Address+"/forward", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		zap.L().Error("Failed to send forward request", zap.Error(err))
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		zap.L().Error("Failed to forward message", zap.String("status", resp.Status))
		return fmt.Errorf("failed to forward message: %s", resp.Status)
	}
	return nil
}

func SendForwardRequestWithRetry(cfg *config.CenterConfig, forwardReq types.ClientMessage, retries int, delayMs int) error {
	var err error
	for i := 0; i < retries; i++ {
		err = SendForwardRequest(cfg, forwardReq)
		if err == nil {
			return nil
		}
		time.Sleep(time.Duration(delayMs) * time.Millisecond)
	}
	return err
}
