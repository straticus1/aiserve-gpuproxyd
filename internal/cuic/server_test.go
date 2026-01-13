package cuic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCUICServer_HandleHeartbeat(t *testing.T) {
	// Create server with nil services for this simple test
	server := NewCUICServer(nil, nil, nil, nil)

	// Test the handleHeartbeat method directly (bypasses auth)
	msg := CUICMessage{
		StreamID:    "test-stream-123",
		MessageID:   "msg-456",
		Version:     "1.0",
		Sender:      "test-client",
		Receiver:    "aiserve-gpuproxy",
		MessageType: MessageTypeHeartbeat,
		Priority:    PriorityNormal,
		Timestamp:   time.Now(),
	}

	ctx := context.Background()

	// Call handleHeartbeat directly
	result, congestion, err := server.handleHeartbeat(ctx, msg)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, CongestionNone, congestion)

	// Verify payload contains expected fields
	payload, ok := result.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "alive", payload["status"])
	assert.Equal(t, msg.StreamID, payload["stream_id"])
}

func TestCUICServer_ValidateMessage(t *testing.T) {
	server := NewCUICServer(nil, nil, nil, nil)

	tests := []struct {
		name    string
		msg     *CUICMessage
		wantErr bool
	}{
		{
			name: "valid message",
			msg: &CUICMessage{
				Sender:      "test-client",
				MessageType: MessageTypeRequest,
			},
			wantErr: false,
		},
		{
			name: "missing sender",
			msg: &CUICMessage{
				MessageType: MessageTypeRequest,
			},
			wantErr: true,
		},
		{
			name: "missing message type",
			msg: &CUICMessage{
				Sender: "test-client",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.validateMessage(tt.msg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify defaults are set
				assert.NotEmpty(t, tt.msg.Version)
				assert.NotEmpty(t, tt.msg.MessageID)
				assert.NotEmpty(t, tt.msg.StreamID)
				assert.NotZero(t, tt.msg.Priority)
			}
		})
	}
}

func TestCUICServer_ControlMessages(t *testing.T) {
	server := NewCUICServer(nil, nil, nil, nil)

	tests := []struct {
		name        string
		controlType string
		wantStatus  string
	}{
		{
			name:        "stream open",
			controlType: "stream.open",
			wantStatus:  "open",
		},
		{
			name:        "stream close",
			controlType: "stream.close",
			wantStatus:  "closed",
		},
		{
			name:        "flow control",
			controlType: "flow.control",
			wantStatus:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := CUICMessage{
				StreamID:    "test-stream",
				MessageID:   "msg-123",
				Sender:      "test-client",
				MessageType: MessageTypeControl,
				Payload: map[string]interface{}{
					"control_type": tt.controlType,
				},
			}

			result, congestion, err := server.handleControl(context.Background(), msg)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			assert.NotEmpty(t, congestion)

			resultMap, ok := result.(map[string]interface{})
			assert.True(t, ok)

			if tt.wantStatus != "" {
				assert.Equal(t, tt.wantStatus, resultMap["status"])
			}
		})
	}
}

func TestCUICMessage_JSONSerialization(t *testing.T) {
	msg := CUICMessage{
		StreamID:    "stream-123",
		MessageID:   "msg-456",
		Version:     "1.0",
		Sender:      "client-1",
		Receiver:    "server-1",
		MessageType: MessageTypeRequest,
		Priority:    PriorityHigh,
		Timestamp:   time.Now(),
		Payload: map[string]interface{}{
			"operation": "gpu.list",
			"parameters": map[string]interface{}{
				"provider": "vast.ai",
			},
		},
		Metadata: map[string]interface{}{
			"trace_id": "trace-789",
		},
		CongestionHint: CongestionNone,
	}

	// Marshal to JSON
	data, err := json.Marshal(msg)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Unmarshal back
	var decoded CUICMessage
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	// Verify fields
	assert.Equal(t, msg.StreamID, decoded.StreamID)
	assert.Equal(t, msg.MessageID, decoded.MessageID)
	assert.Equal(t, msg.Version, decoded.Version)
	assert.Equal(t, msg.Sender, decoded.Sender)
	assert.Equal(t, msg.Receiver, decoded.Receiver)
	assert.Equal(t, msg.MessageType, decoded.MessageType)
	assert.Equal(t, msg.Priority, decoded.Priority)
	assert.Equal(t, msg.CongestionHint, decoded.CongestionHint)
}

func TestCUICServer_ProtocolDetection(t *testing.T) {
	// This test verifies CUIC messages can be detected properly
	cuicMsg := map[string]interface{}{
		"stream_id":    "stream-123",
		"message_id":   "msg-456",
		"sender":       "client",
		"message_type": "request",
		"priority":     128,
	}

	data, err := json.Marshal(cuicMsg)
	assert.NoError(t, err)

	// Verify it has the identifying fields
	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	_, hasStreamID := decoded["stream_id"]
	_, hasPriority := decoded["priority"]
	msgType, _ := decoded["message_type"].(string)

	assert.True(t, hasStreamID, "CUIC messages must have stream_id")
	assert.True(t, hasPriority, "CUIC messages must have priority")
	assert.Contains(t, []string{"stream", "datagram", "request", "response", "control", "heartbeat"}, msgType)
}
