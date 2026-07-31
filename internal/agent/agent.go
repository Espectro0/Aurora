package agent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/conversation"
	"github.com/Espectro0/AuroraProject/internal/identity"
	"github.com/Espectro0/AuroraProject/internal/llm"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/reflection"
)

type Agent struct {
	llm       llm.Provider
	identity  *identity.Core
	memory    memory.Store
	reflector *reflection.Reflector
	msgCount  map[string]int
}

func NewAgent(llm llm.Provider, id *identity.Core, memory memory.Store, reflector *reflection.Reflector) *Agent {
	return &Agent{
		llm:       llm,
		identity:  id,
		memory:    memory,
		reflector: reflector,
		msgCount:  make(map[string]int),
	}
}

func (a *Agent) Reply(ctx context.Context, userID string, message string) (string, error) {
	userMsg := conversation.NewMessage(conversation.User, message)
	a.memory.Save(userID, userMsg)

	identity := a.identity.Get()
	systemContent := buildSystemMessage(identity)

	history := []conversation.Message{
		conversation.NewMessage(conversation.System, systemContent),
	}
	history = append(history, a.memory.History(userID)...)

	response, err := a.llm.Chat(ctx, history)
	if err != nil {
		return "", err
	}

	assistantMsg := conversation.NewMessage(conversation.Assistant, response)
	a.memory.Save(userID, assistantMsg)

	if a.reflector != nil {
		a.msgCount[userID]++
		if a.msgCount[userID] >= a.reflector.Interval() {
			a.msgCount[userID] = 0
			go func() {
				if err := a.reflector.Analyze(context.Background(), userID); err != nil {
					log.Printf("[agent] reflection error: %v", err)
				}
			}()
		}
	}

	return response, nil
}

func buildSystemMessage(id identity.IdentityCore) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Eres %s. %s", id.Name, id.Description))

	if len(id.Values) > 0 {
		b.WriteString("\n\nValores:")
		for _, v := range id.Values {
			b.WriteString(fmt.Sprintf("\n- %s", v))
		}
	}

	if id.Purpose != "" {
		b.WriteString(fmt.Sprintf("\n\nPropósito: %s", id.Purpose))
	}

	if len(id.FoundationalMemories) > 0 {
		b.WriteString("\n\nRecuerdos fundacionales:")
		for _, m := range id.FoundationalMemories {
			b.WriteString(fmt.Sprintf("\n- %s", m))
		}
	}

	if len(id.ConversationalPrinciples) > 0 {
		b.WriteString("\n\nPrincipios:")
		for _, p := range id.ConversationalPrinciples {
			b.WriteString(fmt.Sprintf("\n- %s", p))
		}
	}

	return b.String()
}
