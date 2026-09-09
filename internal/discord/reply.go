package discord

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

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
