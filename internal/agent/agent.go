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
	memStore  memory.MemoryStore
	reflector *reflection.Reflector
	msgCount  map[string]int
}

func NewAgent(llm llm.Provider, id *identity.Core, memory memory.Store, memStore memory.MemoryStore, reflector *reflection.Reflector) *Agent {
	return &Agent{
		llm:       llm,
		identity:  id,
		memory:    memory,
		memStore:  memStore,
		reflector: reflector,
		msgCount:  make(map[string]int),
	}
}

func (a *Agent) Reply(ctx context.Context, userID string, message string) (string, error) {
	userMsg := conversation.NewMessage(conversation.User, message)
	a.memory.Save(userID, userMsg)

	history := []conversation.Message{
		conversation.NewMessage(conversation.System, buildSystemMessage(a.identity.Get())),
	}

	if a.memStore != nil {
		relevant, err := a.memStore.SearchNodes(ctx, message, 5)
		if err != nil {
			log.Printf("[agent] memory search error: %v", err)
		} else {
			threshold := a.identity.Get().MemoryUsageRules.SemanticRelevanceThreshold
			kept := make([]memory.Node, 0, len(relevant))
			for _, n := range relevant {
				if n.Similarity >= threshold {
					kept = append(kept, n)
				}
			}

			if len(kept) > 0 {
				var b strings.Builder
				b.WriteString("Estos son tus recuerdos a largo plazo recuperados en este momento.\n")
				b.WriteString("Si el usuario menciona o pregunta por algo de aqui, responde con naturalidad y seguridad como algo que tu recuerdas.\n")
				b.WriteString("No digas que no recuerdas si la informacion esta aqui.\n\n")
				for _, n := range kept {
					b.WriteString(fmt.Sprintf("- %s\n", n.Content))
				}
				history = append(history, conversation.NewMessage(conversation.System, b.String()))

				maxSim := kept[0].Similarity
				log.Printf("[agent] memories injected: %d (max sim %.2f)", len(kept), maxSim)
			}
		}
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
