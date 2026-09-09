package telegram

import (
	"context"
	"time"

	"github.com/Espectro0/AuroraProject/internal/agent"
	"github.com/Espectro0/AuroraProject/internal/transcription"
)

type Bot struct {
	ctx                  context.Context
	api                  *apiClient
	agent                agent.Service
	transcription        transcription.Provider
	transcriptionTimeout time.Duration
}

func NewBot(token string, agent agent.Service, transcription transcription.Provider, transcriptionTimeout time.Duration) *Bot {
	return &Bot{
		api:                  newAPIClient(token),
		agent:                agent,
		transcription:        transcription,
		transcriptionTimeout: transcriptionTimeout,
	}
}
