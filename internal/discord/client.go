package discord

import (
	"context"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/gateway"
)

func (b *Bot) Run(ctx context.Context) error {
	b.ctx = ctx

	client, err := disgo.New(b.token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuildMessages|gateway.IntentMessageContent)),
		bot.WithEventListenerFunc(b.onMessageCreate),
		bot.WithEventListenerFunc(b.onReady),
	)
	if err != nil {
		return err
	}

	b.client = client

	go func() {
		<-ctx.Done()
		client.Close(ctx)
	}()

	return client.OpenGateway(ctx)
}
