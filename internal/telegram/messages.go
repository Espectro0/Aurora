package telegram

import (
	"log"
	"strconv"
	"strings"
)

func (b *Bot) handleUpdate(u update) {
	if u.Message == nil {
		return
	}
	b.handleMessage(*u.Message)
}

func (b *Bot) handleMessage(m message) {
	userID := "telegram:" + strconv.FormatInt(m.From.ID, 10)

	content := strings.TrimSpace(m.Text)
	if content == "" {
		return
	}

	response, attachments, err := b.agent.Reply(b.ctx, userID, content)
	if err != nil {
		log.Printf("agent error: %v", err)
		b.sendMessage(m.Chat.ID, "Lo siento, ocurrió un error interno.")
		return
	}

	b.sendMessage(m.Chat.ID, response)
	b.sendAttachments(m.Chat.ID, attachments)
}
