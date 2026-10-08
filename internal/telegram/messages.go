package telegram

import (
	"log"
	"strconv"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/conversation"
	"github.com/Espectro0/AuroraProject/internal/guard"
)

func (b *Bot) handleUpdate(u update) {
	switch {
	case u.CallbackQuery != nil:
		b.handleCallback(*u.CallbackQuery)
	case u.Message != nil:
		b.handleMessage(*u.Message)
	}
}

func (b *Bot) handleMessage(m message) {
	if len(b.allowedUsers) > 0 && !b.allowedUsers[strconv.FormatInt(m.From.ID, 10)] {
		return
	}
	userID := "telegram:" + strconv.FormatInt(m.From.ID, 10)

	content := strings.TrimSpace(m.Text)
	if content == "" {
		return
	}

	name := m.From.FirstName
	if name == "" {
		name = m.From.Username
	}
	owner := len(b.allowedUsers) > 0
	ctx := conversation.WithSpeaker(b.ctx, conversation.Speaker{ID: userID, Name: name, Owner: owner})
	ctx = guard.WithConfirmer(ctx, &confirmer{bot: b, chatID: m.Chat.ID, owner: userID})
	response, attachments, err := b.agent.Reply(ctx, userID, content)
	if err != nil {
		log.Printf("agent error: %v", err)
		b.sendMessage(m.Chat.ID, "Lo siento, ocurrió un error interno.")
		return
	}

	b.sendMessage(m.Chat.ID, response)
	b.sendAttachments(m.Chat.ID, attachments)
}
