package tokilake

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sync"

	"one-api/common"
)

const (
	ControlMessageTypeRegister      = "register"
	ControlMessageTypeHeartbeat     = "heartbeat"
	ControlMessageTypeModelsSync    = "models_sync"
	ControlMessageTypeCancelRequest = "cancel_request"
	ControlMessageTypeAck           = "ack"
	ControlMessageTypeError         = "error"
)

type ControlMessage struct {
	Type          string                `json:"type"`
	RequestID     string                `json:"request_id,omitempty"`
	Register      *RegisterMessage      `json:"register,omitempty"`
	Heartbeat     *HeartbeatMessage     `json:"heartbeat,omitempty"`
	ModelsSync    *ModelsSyncMessage    `json:"models_sync,omitempty"`
	CancelRequest *CancelRequestMessage `json:"cancel_request,omitempty"`
	Ack           *AckMessage           `json:"ack,omitempty"`
	Error         *ErrorMessage         `json:"error,omitempty"`
}

type RegisterMessage struct {
	Namespace    string         `json:"namespace"`
	NodeName     string         `json:"node_name,omitempty"`
	Group        string         `json:"group,omitempty"`
	Models       []string       `json:"models,omitempty"`
	HardwareInfo map[string]any `json:"hardware_info,omitempty"`
	BackendType  string         `json:"backend_type,omitempty"`
}

type HeartbeatMessage struct {
	Status        int            `json:"status,omitempty"`
	NodeName      string         `json:"node_name,omitempty"`
	HardwareInfo  map[string]any `json:"hardware_info,omitempty"`
	CurrentModels []string       `json:"current_models,omitempty"`
}

type ModelsSyncMessage struct {
	Group        string         `json:"group,omitempty"`
	Models       []string       `json:"models,omitempty"`
	HardwareInfo map[string]any `json:"hardware_info,omitempty"`
	BackendType  string         `json:"backend_type,omitempty"`
}

type CancelRequestMessage struct {
	TargetRequestID string `json:"target_request_id"`
	Reason          string `json:"reason,omitempty"`
}

type AckMessage struct {
	Message   string `json:"message,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	WorkerID  int    `json:"worker_id,omitempty"`
	ChannelID int    `json:"channel_id,omitempty"`
}

type ErrorMessage struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message"`
}

type controlCodec struct {
	reader  *bufio.Reader
	stream  io.ReadWriteCloser
	writeMu sync.Mutex
}

func newControlCodec(stream io.ReadWriteCloser) *controlCodec {
	return &controlCodec{
		reader: bufio.NewReader(stream),
		stream: stream,
	}
}

func (c *controlCodec) ReadMessage() (*ControlMessage, error) {
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		msg := &ControlMessage{}
		if err = common.Unmarshal(line, msg); err != nil {
			return nil, fmt.Errorf("decode control message: %w", err)
		}
		return msg, nil
	}
}

func (c *controlCodec) WriteMessage(message *ControlMessage) error {
	data, err := common.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode control message: %w", err)
	}
	data = append(data, '\n')

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if _, err = c.stream.Write(data); err != nil {
		return fmt.Errorf("write control message: %w", err)
	}
	return nil
}
