package discord

import (
	"log"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
)

func (b *Bot) onReady(e *events.Ready) {
	log.Printf("[discord] logged in as %s", e.User.Username)

	err := b.client.SetPresence(b.ctx, func(p *gateway.MessageDataPresenceUpdate) {
		p.Activities = []discord.Activity{
			{
				Name: "Decoding your messages...",
				Type: discord.ActivityTypeStreaming,
			},
		}
		p.Status = "dnd"
		p.AFK = false
	})

	if err != nil {
		log.Printf("[discord] error setting presence: %v", err)
	}
}
