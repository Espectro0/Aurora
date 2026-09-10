package discord

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/skills"
	ddiscord "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

const maxMessageLen = 2000

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

func (b *Bot) sendAttachments(e *events.MessageCreate, attachments []skills.Attachment) {
	for _, a := range attachments {
		f, err := os.Open(a.Path)
		if err != nil {
			log.Printf("error opening attachment %s: %v", a.Path, err)
			continue
		}

		name := a.Filename
		if name == "" {
			name = "adjunto"
		}

		_, err = e.Client().Rest.CreateMessage(e.ChannelID, ddiscord.MessageCreate{
			Files: []*ddiscord.File{ddiscord.NewFile(name, "", f)},
		})
		f.Close()
		if err != nil {
			log.Printf("error sending attachment %s: %v", a.Path, err)
		}
	}
}

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
