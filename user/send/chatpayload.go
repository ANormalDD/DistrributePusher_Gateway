package send

import (
	"encoding/json"
	"fmt"
)

type ChatPayload interface {
	GetType() string
}

type TextPayload struct {
	Text string `json:"text"`
}

type ImagePayload struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type VoicePayload struct {
	URL      string `json:"url"`
	Duration int    `json:"duration"` // 秒数
}

type FilePayload struct {
	URL      string `json:"url"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size"` // 字节
}

func (t TextPayload) GetType() string {
	return "text"
}

func (i ImagePayload) GetType() string {
	return "image"
}

func (v VoicePayload) GetType() string {
	return "voice"
}

func (f FilePayload) GetType() string {
	return "file"
}

// UnmarshalChatPayload 根据 content JSON 内的 "type" 字段反序列化为具体的 payload
func UnmarshalChatPayload(data json.RawMessage) (ChatPayload, error) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("invalid content json: %w", err)
	}
	switch probe.Type {
	case "text":
		var p TextPayload
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("invalid text payload: %w", err)
		}
		return p, nil
	case "image":
		var p ImagePayload
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("invalid image payload: %w", err)
		}
		return p, nil
	case "voice":
		var p VoicePayload
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("invalid voice payload: %w", err)
		}
		return p, nil
	case "file":
		var p FilePayload
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("invalid file payload: %w", err)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("unknown content type: %s", probe.Type)
	}
}
