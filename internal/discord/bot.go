package discord

import (
	"context"

	"github.com/Espectro0/AuroraProject/internal/agent"

	"github.com/disgoorg/disgo/bot"
)

type Bot struct {
	ctx    context.Context
	token  string
	agent  agent.Service
	client *bot.Client
}

func NewBot(token string, agent agent.Service) *Bot {
	return &Bot{
		token: token,
		agent: agent,
	}
}
