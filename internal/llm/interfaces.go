package llm

import (
	"context"

	"github.com/Espectro0/AuroraProject/internal/conversation"
)

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type ChatResult struct {
	Content   string
	ToolCalls []conversation.ToolCall
}

type Provider interface {
	Chat(ctx context.Context, message []conversation.Message) (string, error)
	ChatWithTools(ctx context.Context, message []conversation.Message, tools []ToolDefinition) (ChatResult, error)
}
