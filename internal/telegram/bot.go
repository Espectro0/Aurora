package telegram

import (
	"context"

	"github.com/Espectro0/AuroraProject/internal/agent"
	"github.com/Espectro0/AuroraProject/internal/guard"
)

type Bot struct {
	ctx          context.Context
	api          *apiClient
	agent        agent.Service
	allowedUsers map[string]bool
	pending      *guard.Pending
}

func NewBot(token string, agent agent.Service, allowedUserIDs []string) *Bot {
	allowed := make(map[string]bool, len(allowedUserIDs))
	for _, id := range allowedUserIDs {
		allowed[id] = true
	}
	return &Bot{
		api:          newAPIClient(token),
		agent:        agent,
		allowedUsers: allowed,
		pending:      guard.NewPending(),
	}
}
