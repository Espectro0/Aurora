package agent

import (
	"context"
)

type Service interface {
	Reply(ctx context.Context, userId string, message string) (string, error)
}
