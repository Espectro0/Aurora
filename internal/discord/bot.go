package discord

import (
	"context"

	"github.com/Espectro0/AuroraProject/internal/agent"
	"github.com/Espectro0/AuroraProject/internal/guard"

	"github.com/disgoorg/disgo/bot"
)

type Bot struct {
	ctx          context.Context
	token        string
	agent        agent.Service
	client       *bot.Client
	allowedUsers map[string]bool
	pending      *guard.Pending
}

func NewBot(token string, agent agent.Service, allowedUserIDs []string) *Bot {
	allowed := make(map[string]bool, len(allowedUserIDs))
	for _, id := range allowedUserIDs {
		allowed[id] = true
	}
	return &Bot{
		token:        token,
		agent:        agent,
		allowedUsers: allowed,
		pending:      guard.NewPending(),
	}
}
