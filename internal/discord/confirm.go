package discord

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/guard"
	ddiscord "github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
)

const confirmPrefix = "guard:"

// confirmer asks the author of a message, in its channel, to approve a guarded skill call.
type confirmer struct {
	bot       *Bot
	channelID snowflake.ID
	owner     string
}

func (c *confirmer) Confirm(ctx context.Context, req guard.Request) (bool, error) {
	id, answer := c.bot.pending.Add(c.owner)
	defer c.bot.pending.Drop(id)
	text := clipRunes(req.Text(), maxMessageLen-100)

	msg, err := c.bot.client.Rest.CreateMessage(c.channelID, ddiscord.MessageCreate{
		Content: text,
		Components: []ddiscord.LayoutComponent{
			ddiscord.NewActionRow(
				ddiscord.NewSuccessButton("Aprobar", confirmPrefix+"yes:"+id),
				ddiscord.NewDangerButton("Rechazar", confirmPrefix+"no:"+id),
			),
		},
	})
	if err != nil {
		return false, err
	}

	ok, err := guard.Await(ctx, answer)
	if err != nil {
		status := "⌛ Confirmación expirada."
		if !errors.Is(err, context.DeadlineExceeded) {
			status = "⚠️ Confirmación cancelada."
		}
		content := text + "\n\n" + status
		empty := []ddiscord.LayoutComponent{}
		if _, uerr := c.bot.client.Rest.UpdateMessage(c.channelID, msg.ID, ddiscord.MessageUpdate{Content: &content, Components: &empty}); uerr != nil {
			log.Printf("[discord] closing confirmation: %v", uerr)
		}
	}
	return ok, err
}

func (b *Bot) onComponent(e *events.ComponentInteractionCreate) {
	custom := e.Data.CustomID()
	if !strings.HasPrefix(custom, confirmPrefix) {
		return
	}
	answer, id, found := strings.Cut(strings.TrimPrefix(custom, confirmPrefix), ":")
	if !found {
		return
	}
	approved := answer == "yes"

	switch err := b.pending.Resolve(id, discordUserID(e.User().ID), approved); {
	case errors.Is(err, guard.ErrConfirmNotOwner):
		b.ephemeral(e, "Solo quien pidió la acción puede confirmarla.")
		return
	case err != nil:
		b.ephemeral(e, "Esta confirmación ya expiró.")
		return
	}

	status := "❌ Rechazado."
	if approved {
		status = "✅ Aprobado."
	}
	content := e.Message.Content + "\n\n" + status
	empty := []ddiscord.LayoutComponent{}
	if err := e.UpdateMessage(ddiscord.MessageUpdate{Content: &content, Components: &empty}); err != nil {
		log.Printf("[discord] updating confirmation: %v", err)
	}
}

func (b *Bot) ephemeral(e *events.ComponentInteractionCreate, content string) {
	if err := e.CreateMessage(ddiscord.MessageCreate{Content: content, Flags: ddiscord.MessageFlagEphemeral}); err != nil {
		log.Printf("[discord] interaction reply: %v", err)
	}
}

func clipRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
