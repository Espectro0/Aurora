package discord

import (
	"context"
	"log"

	"github.com/Espectro0/AuroraProject/internal/agent"
	"github.com/bwmarrin/discordgo"
)

type Bot struct {
	token string
	agent agent.Service
}

func NewBot(token string, agent agent.Service) *Bot {
	return &Bot{
		token: token,
		agent: agent,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	session, err := discordgo.New("Bot " + b.token)
	if err != nil {
		return err
	}

	session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}

		response, err := b.agent.Reply(ctx, m.Author.ID, m.Content)
		if err != nil {
			log.Printf("agent error: %v", err)
			s.ChannelMessageSend(m.ChannelID, "Lo siento, ocurrió un error interno.")
			return
		}

		s.ChannelMessageSend(m.ChannelID, response)
	})

	session.Identify.Intents = discordgo.IntentGuildMessages | discordgo.IntentsMessageContent

	return session.Open()
}
