package centerclient

// centerclient provides a small indirection layer so other packages (eg. pkg/push)
// can call into the center client without importing the top-level center_client package,
// which would create an import cycle.

import (
	"Gateway/pkg/config"
	"Gateway/pkg/push/types"
	"errors"
)

// SendPushBackFunc is the function signature used to send a pushback request to center.
type SendPushBackFunc func(cfg *config.CenterConfig, forwardReq types.ClientMessage, targetID int64) error

var (
	// default implementation returns an error indicating no sender registered
	sendPushBackFunc SendPushBackFunc = func(cfg *config.CenterConfig, forwardReq types.ClientMessage, targetID int64) error {
		return errors.New("no center client sender registered")
	}
)

// RegisterSender registers the concrete function used to perform SendPushBackRequest.
// Typically called by the `center_client` package during startup.
func RegisterSender(f SendPushBackFunc) {
	if f == nil {
		return
	}
	sendPushBackFunc = f
}

// SendPushBackRequest delegates to the registered sender implementation.
func SendPushBackRequest(cfg *config.CenterConfig, forwardReq types.ClientMessage, targetID int64) error {
	return sendPushBackFunc(cfg, forwardReq, targetID)
}
