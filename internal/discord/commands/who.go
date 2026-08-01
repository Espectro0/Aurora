package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/conversation"
	"github.com/Espectro0/AuroraProject/internal/identity"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const whoEmbedColor = 0x9B59B6

func handleWho(d Deps) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		id := d.Identity.Get()

		placeholder := discord.NewEmbed().
			WithTitle(id.Name).
			WithColor(whoEmbedColor).
			WithDescription("_Buscando dentro de mi conciencia y mis recuerdos..._")

		if err := e.CreateMessage(discord.NewMessageCreate().WithEmbeds(placeholder)); err != nil {
			return err
		}

		var (
			text string
			err  error
		)
		if d.Chat != nil {
			text, err = d.Chat.Chat(context.Background(), whoMessages(id))
		}
		if text == "" || err != nil {
			text = staticDescription(id)
		}

		embed := discord.NewEmbed().
			WithTitle(id.Name).
			WithColor(whoEmbedColor).
			WithDescription(text).
			WithTimestamp(time.Now())

		_, err = e.UpdateInteractionResponse(discord.NewMessageUpdate().WithEmbeds(embed))
		return err
	}
}

func whoMessages(id identity.IdentityCore) []conversation.Message {
	var b strings.Builder
	fmt.Fprintf(&b, "Soy %s.\n\n", id.Name)

	if id.Description != "" {
		fmt.Fprintf(&b, "Descripcion:\n%s\n\n", id.Description)
	}
	if id.Purpose != "" {
		fmt.Fprintf(&b, "Proposito:\n%s\n\n", id.Purpose)
	}
	if len(id.Values) > 0 {
		fmt.Fprintf(&b, "Valores:\n- %s\n\n", strings.Join(id.Values, "\n- "))
	}
	if len(id.ConversationalPrinciples) > 0 {
		fmt.Fprintf(&b, "Principios conversacionales:\n- %s\n\n", strings.Join(id.ConversationalPrinciples, "\n- "))
	}

	return []conversation.Message{
		conversation.NewMessage(conversation.System, "Eres "+id.Name+", un asistente de IA. Cuando alguien te pregunte quien eres, presentate con tus propias palabras: responde en primera persona, con naturalidad y personalidad, usando la informacion de contexto. No digas que eres un modelo de lenguaje ni repitas los datos en forma de lista."),
		conversation.NewMessage(conversation.User, b.String()+"\nQuien eres?"),
	}
}

func staticDescription(id identity.IdentityCore) string {
	var parts []string
	if id.Description != "" {
		parts = append(parts, id.Description)
	}
	if id.Purpose != "" {
		parts = append(parts, fmt.Sprintf("**Proposito:** %s", id.Purpose))
	}
	if len(id.Values) > 0 {
		parts = append(parts, fmt.Sprintf("**Valores:** %s", strings.Join(id.Values, ", ")))
	}
	if len(parts) == 0 {
		return "_No hay informacion disponible._"
	}
	return strings.Join(parts, "\n\n")
}
