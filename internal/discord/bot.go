package discord

import (
	"context"

	"github.com/Espectro0/AuroraProject/internal/agent"
	"github.com/Espectro0/AuroraProject/internal/guard"

	"github.com/disgoorg/disgo/bot"
)

type Bot struct {
	ctx         context.Context
	token       string
	agent       agent.Service
	client      *bot.Client
	ownerID     string
	homeGuildID string
	pending     *guard.Pending
}

func NewBot(token string, agent agent.Service, ownerID, homeGuildID string) *Bot {
	return &Bot{
		token:       token,
		agent:       agent,
		ownerID:     ownerID,
		homeGuildID: homeGuildID,
		pending:     guard.NewPending(),
	}
}
