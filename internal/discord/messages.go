package discord

import (
	"log"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/conversation"
	"github.com/Espectro0/AuroraProject/internal/guard"

	ddiscord "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func (b *Bot) onMessageCreate(e *events.MessageCreate) {
	author := e.Message.Author
	if author.ID == b.client.ID() {
		return
	}

	isOwner := !author.Bot && b.ownerID != "" && author.ID.String() == b.ownerID
	guildID := e.Message.GuildID
	ownerFree := isOwner && (b.homeGuildID == "" || (guildID != nil && guildID.String() == b.homeGuildID))
	if guildID != nil && !ownerFree && !b.addressedToBot(e.Message) {
		return
	}

	content := strings.TrimSpace(b.resolveMentions(e.Message))
	if content == "" {
		return
	}

	userID := discordUserID(author.ID)
	ctx := conversation.WithSpeaker(b.ctx, conversation.Speaker{
		ID:      userID,
		Name:    authorName(e.Message),
		Owner:   isOwner,
		Bot:     author.Bot,
		Mention: author.Mention(),
	})
	ctx = guard.WithConfirmer(ctx, &confirmer{bot: b, channelID: e.ChannelID, owner: userID})
	response, attachments, err := b.agent.Reply(ctx, userID, content)
	if err != nil {
		log.Printf("agent error: %v", err)
		if _, err := e.Client().Rest.CreateMessage(e.ChannelID, ddiscord.MessageCreate{Content: "Lo siento, ocurrió un error interno.", AllowedMentions: &userMentionsOnly}); err != nil {
			log.Printf("error sending message: %v", err)
		}
		return
	}

	b.sendMessage(e, response)
	b.sendAttachments(e, attachments)
}

func (b *Bot) addressedToBot(m ddiscord.Message) bool {
	for _, u := range m.Mentions {
		if u.ID == b.client.ID() {
			return true
		}
	}
	return m.ReferencedMessage != nil && m.ReferencedMessage.Author.ID == b.client.ID()
}

func (b *Bot) resolveMentions(m ddiscord.Message) string {
	self := b.client.ID().String()
	pairs := []string{"<@" + self + ">", "", "<@!" + self + ">", ""}
	for _, u := range m.Mentions {
		if u.ID == b.client.ID() {
			continue
		}
		named := "@" + u.EffectiveName() + " (" + u.Mention() + ")"
		pairs = append(pairs, "<@"+u.ID.String()+">", named, "<@!"+u.ID.String()+">", named)
	}
	return strings.NewReplacer(pairs...).Replace(m.Content)
}

func authorName(m ddiscord.Message) string {
	if m.Member != nil && m.Member.Nick != nil {
		return *m.Member.Nick
	}
	return m.Author.EffectiveName()
}

func discordUserID(id snowflake.ID) string {
	return "discord:" + id.String()
}
