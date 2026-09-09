package telegram

import (
	"fmt"
	"log"
	"strings"
)

const maxMessageLen = 4096

func (b *Bot) sendMessage(chatID int64, content string) {
	chunks := chunkText(content, maxMessageLen)
	if len(chunks) == 0 {
		return
	}
	if len(chunks) == 1 {
		b.sendChunk(chatID, chunks[0])
		return
	}

	total := len(chunks)
	for i, chunk := range chunks {
		prefix := fmt.Sprintf("[%d/%d] ", i+1, total)
		room := maxMessageLen - len([]rune(prefix))
		if len([]rune(chunk)) > room {
			chunk = string([]rune(chunk)[:room])
		}
		b.sendChunk(chatID, prefix+chunk)
	}
}

func (b *Bot) sendChunk(chatID int64, content string) {
	if err := b.api.sendMessage(b.ctx, chatID, content); err != nil {
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
