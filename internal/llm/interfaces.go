package llm

import (
	"context"

	"github.com/Espectro0/AuroraProject/internal/conversation"
)

type Provider interface {
	Chat(ctx context.Context, message []conversation.Message) (string, error)
}
