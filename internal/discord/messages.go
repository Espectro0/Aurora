package discord

import (
	"context"
	"log"
	"path/filepath"
	"strings"

	ddiscord "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

func (b *Bot) onMessageCreate(e *events.MessageCreate) {
	if e.Message.Author.ID == b.client.ID() {
		return
	}

	if att := audioAttachment(e.Message); att != nil {
		go b.handleAudioMessage(e, att)
		return
	}

	content := strings.TrimSpace(e.Message.Content)
	if content == "" {
		return
	}

	response, err := b.agent.Reply(b.ctx, discordUserID(e.Message.Author.ID), content)
	if err != nil {
		log.Printf("agent error: %v", err)
		if _, err := e.Client().Rest.CreateMessage(e.ChannelID, ddiscord.MessageCreate{Content: "Lo siento, ocurrió un error interno."}); err != nil {
			log.Printf("error sending message: %v", err)
		}
		return
	}

	b.sendMessage(e, response)
}

func (b *Bot) handleAudioMessage(e *events.MessageCreate, att *ddiscord.Attachment) {
	_ = e.Client().Rest.SendTyping(e.ChannelID)

	ctx, cancel := context.WithTimeout(b.ctx, b.transcriptionTimeout)
	defer cancel()

	data, err := downloadAttachment(ctx, att)
	if err != nil {
		log.Printf("error downloading audio: %v", err)
		_, _ = e.Client().Rest.CreateMessage(e.ChannelID, ddiscord.MessageCreate{Content: "No pude descargar tu audio."})
		return
	}

	text, err := b.transcription.Transcribe(ctx, data, att.Filename)
	if err != nil {
		log.Printf("transcription error: %v", err)
		_, _ = e.Client().Rest.CreateMessage(e.ChannelID, ddiscord.MessageCreate{Content: "No pude transcribir tu audio."})
		return
	}

	text = strings.TrimSpace(text)
	if text == "" {
		_, _ = e.Client().Rest.CreateMessage(e.ChannelID, ddiscord.MessageCreate{Content: "No pude transcribir tu audio."})
		return
	}

	log.Printf("[transcript] Transcribed Text: %s", text)

	response, err := b.agent.Reply(b.ctx, discordUserID(e.Message.Author.ID), text)
	if err != nil {
		log.Printf("agent error: %v", err)
		if _, err := e.Client().Rest.CreateMessage(e.ChannelID, ddiscord.MessageCreate{Content: "Lo siento, ocurrió un error interno."}); err != nil {
			log.Printf("error sending message: %v", err)
		}
		return
	}

	b.sendMessage(e, response)
}

func discordUserID(id snowflake.ID) string {
	return "discord:" + id.String()
}

func audioAttachment(m ddiscord.Message) *ddiscord.Attachment {
	for _, a := range m.Attachments {
		if a.ContentType != nil && strings.HasPrefix(strings.ToLower(*a.ContentType), "audio/") {
			return &a
		}
		switch strings.ToLower(filepath.Ext(a.Filename)) {
		case ".ogg", ".opus", ".mp3", ".wav", ".flac", ".m4a", ".mp4", ".aac":
			return &a
		}
	}
	return nil
}
