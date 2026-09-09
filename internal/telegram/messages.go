package telegram

import (
	"context"
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

	if m.Voice != nil || m.Audio != nil {
		b.handleAudioMessage(m, userID)
		return
	}

	content := strings.TrimSpace(m.Text)
	if content == "" {
		return
	}

	response, err := b.agent.Reply(b.ctx, userID, content)
	if err != nil {
		log.Printf("agent error: %v", err)
		b.sendMessage(m.Chat.ID, "Lo siento, ocurrió un error interno.")
		return
	}

	b.sendMessage(m.Chat.ID, response)
}

func (b *Bot) handleAudioMessage(m message, userID string) {
	_ = b.api.sendChatAction(b.ctx, m.Chat.ID, "typing")

	ctx, cancel := context.WithTimeout(b.ctx, b.transcriptionTimeout)
	defer cancel()

	fileID := ""
	switch {
	case m.Voice != nil:
		fileID = m.Voice.FileID
	case m.Audio != nil:
		fileID = m.Audio.FileID
	}

	f, err := b.api.getFile(ctx, fileID)
	if err != nil {
		log.Printf("error getting file info: %v", err)
		b.sendMessage(m.Chat.ID, "No pude descargar tu audio.")
		return
	}

	data, err := b.api.downloadFile(ctx, f.FilePath)
	if err != nil {
		log.Printf("error downloading audio: %v", err)
		b.sendMessage(m.Chat.ID, "No pude descargar tu audio.")
		return
	}

	text, err := b.transcription.Transcribe(ctx, data, f.FilePath)
	if err != nil {
		log.Printf("transcription error: %v", err)
		b.sendMessage(m.Chat.ID, "No pude transcribir tu audio.")
		return
	}

	text = strings.TrimSpace(text)
	if text == "" {
		b.sendMessage(m.Chat.ID, "No pude transcribir tu audio.")
		return
	}

	log.Printf("[transcript] Transcribed Text: %s", text)

	response, err := b.agent.Reply(b.ctx, userID, text)
	if err != nil {
		log.Printf("agent error: %v", err)
		b.sendMessage(m.Chat.ID, "Lo siento, ocurrió un error interno.")
		return
	}

	b.sendMessage(m.Chat.ID, response)
}
