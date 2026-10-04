package memory

import (
	"sync"

	"github.com/Espectro0/AuroraProject/internal/conversation"
)

type InMemory struct {
	mu            sync.RWMutex
	conversations map[string][]conversation.Message
}

func NewInMemory() *InMemory {
	return &InMemory{conversations: make(map[string][]conversation.Message)}
}

func (m *InMemory) Save(userID string, msg conversation.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.conversations[userID] = append(m.conversations[userID], msg)
	return nil
}

func (m *InMemory) History(userID string) []conversation.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	msgs := m.conversations[userID]
	out := make([]conversation.Message, len(msgs))
	copy(out, msgs)
	return out
}
