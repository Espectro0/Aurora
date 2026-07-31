package memory

import (
	"github.com/Espectro0/AuroraProject/internal/conversation"
)

type InMemory struct {
	conversations map[string][]conversation.Message
}

func NewInMemory() *InMemory {
	return &InMemory{conversations: make(map[string][]conversation.Message)}
}

func (m *InMemory) Save(userID string, msg conversation.Message) error {
	m.conversations[userID] = append(m.conversations[userID], msg)
	return nil
}

func (m *InMemory) History(userID string) []conversation.Message {
	return m.conversations[userID]
}
