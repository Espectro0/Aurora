package discord

import (
	"log"
	"strings"

	ddiscord "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func (b *Bot) onMessageCreate(e *events.MessageCreate) {
	if e.Message.Author.ID == b.client.ID() {
		return
	}

	content := strings.TrimSpace(e.Message.Content)
	if content == "" {
		return
	}

	response, attachments, err := b.agent.Reply(b.ctx, discordUserID(e.Message.Author.ID), content)
	if err != nil {
		log.Printf("agent error: %v", err)
		if _, err := e.Client().Rest.CreateMessage(e.ChannelID, ddiscord.MessageCreate{Content: "Lo siento, ocurrió un error interno."}); err != nil {
			log.Printf("error sending message: %v", err)
		}
		return
	}

	b.sendMessage(e, response)
	b.sendAttachments(e, attachments)
}

func discordUserID(id snowflake.ID) string {
	return "discord:" + id.String()
}
