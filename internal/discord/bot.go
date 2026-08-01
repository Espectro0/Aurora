package discord

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/agent"
	"github.com/Espectro0/AuroraProject/internal/discord/commands"
	"github.com/Espectro0/AuroraProject/internal/transcription"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	ddiscord "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
)

type Bot struct {
	ctx                  context.Context
	token                string
	agent                agent.Service
	transcription        transcription.Provider
	transcriptionTimeout time.Duration
	commands             *commands.Commands
	client               *bot.Client
}

func NewBot(token string, agent agent.Service, transcription transcription.Provider, transcriptionTimeout time.Duration, commands *commands.Commands) *Bot {
	return &Bot{
		token:                token,
		agent:                agent,
		transcription:        transcription,
		transcriptionTimeout: transcriptionTimeout,
		commands:             commands,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	b.ctx = ctx

	client, err := disgo.New(b.token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildMessages|gateway.IntentMessageContent)),
		bot.WithEventListenerFunc(b.onMessageCreate),
	)
	if err != nil {
		return err
	}

	if b.commands != nil {
		client.AddEventListeners(b.commands.Mux())
		if _, err := client.Rest.SetGlobalCommands(client.ApplicationID, b.commands.Definitions()); err != nil {
			log.Printf("[discord] error registering commands: %v", err)
		}
	}
	b.client = client

	go func() {
		<-ctx.Done()
		client.Close(ctx)
	}()

	return client.OpenGateway(ctx)
}

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

	response, err := b.agent.Reply(b.ctx, e.Message.Author.ID.String(), content)
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

	response, err := b.agent.Reply(b.ctx, e.Message.Author.ID.String(), text)
	if err != nil {
		log.Printf("agent error: %v", err)
		if _, err := e.Client().Rest.CreateMessage(e.ChannelID, ddiscord.MessageCreate{Content: "Lo siento, ocurrió un error interno."}); err != nil {
			log.Printf("error sending message: %v", err)
		}
		return
	}

	b.sendMessage(e, response)
}

const maxMessageLen = 2000

func chunkText(s string, max int) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	runes := []rune(s)
	var chunks []string
	for len(runes) > max {
		cut := max
		if idx := strings.LastIndex(string(runes[:max]), "\n"); idx > 0 {
			cut = idx + 1
		}
		chunks = append(chunks, strings.TrimSpace(string(runes[:cut])))
		runes = runes[cut:]
	}
	if len(runes) > 0 {
		chunks = append(chunks, strings.TrimSpace(string(runes)))
	}
	return chunks
}

func (b *Bot) sendMessage(e *events.MessageCreate, content string) {
	chunks := chunkText(content, maxMessageLen)
	if len(chunks) == 0 {
		return
	}
	if len(chunks) == 1 {
		b.sendChunk(e, chunks[0])
		return
	}

	total := len(chunks)
	for i, chunk := range chunks {
		prefix := fmt.Sprintf("[%d/%d] ", i+1, total)
		room := maxMessageLen - len([]rune(prefix))
		if len([]rune(chunk)) > room {
			chunk = string([]rune(chunk)[:room])
		}
		b.sendChunk(e, prefix+chunk)
	}
}

func (b *Bot) sendChunk(e *events.MessageCreate, content string) {
	if _, err := e.Client().Rest.CreateMessage(e.ChannelID, ddiscord.MessageCreate{Content: content}); err != nil {
		log.Printf("error sending message: %v", err)
	}
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

func downloadAttachment(ctx context.Context, att *ddiscord.Attachment) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, att.URL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
