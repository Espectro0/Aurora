package discord

import (
	"log"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/guard"

	ddiscord "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func (b *Bot) onMessageCreate(e *events.MessageCreate) {
	if e.Message.Author.ID == b.client.ID() {
		return
	}
	if len(b.allowedUsers) > 0 && !b.allowedUsers[e.Message.Author.ID.String()] {
		return
	}

	content := strings.TrimSpace(e.Message.Content)
	if content == "" {
		return
	}

	userID := discordUserID(e.Message.Author.ID)
	ctx := guard.WithConfirmer(b.ctx, &confirmer{bot: b, channelID: e.ChannelID, owner: userID})
	response, attachments, err := b.agent.Reply(ctx, userID, content)
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
