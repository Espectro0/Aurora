package agent

import (
	"context"

	"github.com/Espectro0/AuroraProject/internal/skills"
)

type Service interface {
	Reply(ctx context.Context, userId string, message string) (string, []skills.Attachment, error)
}
