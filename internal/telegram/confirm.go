package telegram

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/guard"
)

const confirmPrefix = "guard:"

type confirmer struct {
	bot    *Bot
	chatID int64
	owner  string
}

func (c *confirmer) Confirm(ctx context.Context, req guard.Request) (bool, error) {
	id, answer := c.bot.pending.Add(c.owner)
	defer c.bot.pending.Drop(id)
	text := clipRunes(req.Text(), maxMessageLen-100)

	msg, err := c.bot.api.sendMessageWithButtons(ctx, c.chatID, text, []inlineButton{
		{Text: "✅ Aprobar", CallbackData: confirmPrefix + "yes:" + id},
		{Text: "❌ Rechazar", CallbackData: confirmPrefix + "no:" + id},
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
		if eerr := c.bot.api.editMessageText(c.bot.ctx, c.chatID, msg.MessageID, text+"\n\n"+status); eerr != nil {
			log.Printf("[telegram] closing confirmation: %v", eerr)
		}
	}
	return ok, err
}

func (b *Bot) handleCallback(q callbackQuery) {
	if !strings.HasPrefix(q.Data, confirmPrefix) {
		return
	}
	answer, id, found := strings.Cut(strings.TrimPrefix(q.Data, confirmPrefix), ":")
	if !found {
		return
	}
	approved := answer == "yes"

	reply := ""
	switch err := b.pending.Resolve(id, "telegram:"+strconv.FormatInt(q.From.ID, 10), approved); {
	case errors.Is(err, guard.ErrConfirmNotOwner):
		reply = "Solo quien pidió la acción puede confirmarla."
	case err != nil:
		reply = "Esta confirmación ya expiró."
	}

	if err := b.api.answerCallbackQuery(b.ctx, q.ID, reply); err != nil {
		log.Printf("[telegram] answerCallbackQuery: %v", err)
	}
	if reply != "" || q.Message == nil {
		return
	}

	status := "❌ Rechazado."
	if approved {
		status = "✅ Aprobado."
	}
	if err := b.api.editMessageText(b.ctx, q.Message.Chat.ID, q.Message.MessageID, q.Message.Text+"\n\n"+status); err != nil {
		log.Printf("[telegram] updating confirmation: %v", err)
	}
}

func clipRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
