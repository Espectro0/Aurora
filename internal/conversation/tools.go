package conversation

import (
	"time"

	"github.com/google/uuid"
)

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

func NewAssistantToolCallMessage(calls []ToolCall) Message {
	return Message{
		ID:        uuid.New(),
		Role:      Assistant,
		ToolCalls: calls,
		CreatedAt: time.Now(),
	}
}

func NewToolResultMessage(callID, result string) Message {
	return Message{
		ID:         uuid.New(),
		Role:       Tool,
		Content:    result,
		ToolCallID: callID,
		CreatedAt:  time.Now(),
	}
}
