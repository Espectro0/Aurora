package telegram

import (
	"context"

	"github.com/Espectro0/AuroraProject/internal/agent"
)

type Bot struct {
	ctx   context.Context
	api   *apiClient
	agent agent.Service
}

func NewBot(token string, agent agent.Service) *Bot {
	return &Bot{
		api:   newAPIClient(token),
		agent: agent,
	}
}
