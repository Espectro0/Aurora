package telegram

import (
	"context"
	"fmt"
	"log"
	"time"
)

const pollTimeoutSeconds = 30

func (b *Bot) Run(ctx context.Context) error {
	b.ctx = ctx

	me, err := b.api.getMe(ctx)
	if err != nil {
		return fmt.Errorf("telegram: getMe: %w", err)
	}
	log.Printf("[telegram] logged in as @%s", me.Username)

	go b.pollLoop(ctx)

	return nil
}

func (b *Bot) pollLoop(ctx context.Context) {
	var offset int64

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := b.api.getUpdates(ctx, offset, pollTimeoutSeconds)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("[telegram] getUpdates error: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		for _, u := range updates {
			offset = u.UpdateID + 1
			go b.handleUpdate(u)
		}
	}
}
