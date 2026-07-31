package conversation

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	System    Role = "system"
	User      Role = "user"
	Assistant Role = "assistant"
)

type Message struct {
	ID        uuid.UUID
	Role      Role
	Content   string
	CreatedAt time.Time
}

func NewMessage(role Role, content string) Message {
	return Message{
		ID:        uuid.New(),
		Role:      role,
		Content:   content,
		CreatedAt: time.Now(),
	}
}
