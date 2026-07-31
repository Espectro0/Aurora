package memory

import "github.com/Espectro0/AuroraProject/internal/conversation"

type Store interface {
	Save(userID string, message conversation.Message) error

	History(userId string) []conversation.Message
}
