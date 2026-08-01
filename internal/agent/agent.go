package agent

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

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

	reflectMu sync.Mutex
	reflectWG sync.WaitGroup

	interestsMu         sync.Mutex
	interestsAt         time.Time
	interestCache       [][]memory.Node
	interestsRefreshing bool
	lastReflectionMu    sync.Mutex
	lastReflection      memory.Node
	lastReflectionAt    time.Time
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
		rules := a.identity.Get().MemoryUsageRules

		limit := rules.MaxContextMemories
		if limit <= 0 {
			limit = 5
		}

		relevant, err := a.searchMemories(ctx, message, limit)
		if err != nil {
			log.Printf("[agent] memory search error: %v", err)
		} else {
			kept := make([]memory.Node, 0, len(relevant))
			for _, n := range relevant {
				if n.Similarity >= rules.SemanticRelevanceThreshold {
					kept = append(kept, n)
				}
			}

			if len(kept) == 0 && len(relevant) > 0 {
				fallback := relevant
				if len(fallback) > 2 {
					fallback = fallback[:2]
				}
				kept = fallback
				log.Printf("[agent] recall fallback: %d (max score %.2f)", len(kept), kept[0].Similarity)
			}

			if rules.RecencyWeight > 0 && len(kept) > 1 {
				applyRecency(kept, rules.RecencyWeight)
			}

			if a.memStore != nil {
				if latest := a.latestReflection(ctx); !latest.CreatedAt.IsZero() {
					history = append(history, conversation.NewMessage(conversation.System, fmt.Sprintf(
						"Contexto de la última conversación recordada: %s", latest.Content)))
					log.Printf("[agent] latest reflection injected")
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

				log.Printf("[agent] memories injected: %d (max score %.2f)", len(kept), kept[0].Similarity)
			}
		}
	}

	history = append(history, a.memory.History(userID)...)

	if a.memStore != nil {
		if clusters := a.interests(ctx); len(clusters) > 0 {
			var b strings.Builder
			b.WriteString("Intereses emergentes: temas sobre los que has hablado con frecuencia y te interesan.\n")
			b.WriteString("Si el usuario menciona alguno de estos temas, puedes responder con naturalidad y profundidad.\n\n")
			for _, c := range clusters {
				b.WriteString(fmt.Sprintf("- %s (%d recuerdos)\n", interestLabel(c), len(c)))
			}
			history = append(history, conversation.NewMessage(conversation.System, b.String()))
			log.Printf("[agent] interests injected: %d", len(clusters))
		}
	}

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
			a.reflectWG.Add(1)
			go func() {
				defer a.reflectWG.Done()
				a.reflectMu.Lock()
				defer a.reflectMu.Unlock()
				if err := a.reflector.Analyze(context.Background(), userID); err != nil {
					log.Printf("[agent] reflection error: %v", err)
				}
			}()
		}
	}

	return response, nil
}

func (a *Agent) Wait() {
	a.reflectWG.Wait()
}

func (a *Agent) searchMemories(ctx context.Context, query string, limit int) ([]memory.Node, error) {
	timeout := a.identity.Get().LLM.EmbedderTimeoutSeconds + 5
	log.Printf("[agent] Searching in Memories...")
	if timeout <= 0 {
		timeout = 35
	}

	var lastErr error

	for attempt := 0; attempt < 2; attempt++ {
		searchCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		nodes, err := a.memStore.SearchNodes(searchCtx, query, limit)
		cancel()

		if err == nil {
			return nodes, nil
		}

		lastErr = err
		log.Printf("[agent] memory search error (intento %d/2): %v", attempt+1, err)

		if isTimeout(err) {
			return nil, err
		}
	}

	return nil, lastErr
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
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

func (a *Agent) interests(ctx context.Context) [][]memory.Node {
	a.interestsMu.Lock()
	defer a.interestsMu.Unlock()

	rules := a.identity.Get().MemoryUsageRules

	ttl := time.Duration(rules.InterestTTLMinutes) * time.Minute
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}

	if time.Since(a.interestsAt) < ttl || a.interestsRefreshing {
		return a.interestCache
	}

	if ctx.Err() != nil {
		return a.interestCache
	}

	a.interestsRefreshing = true
	a.reflectWG.Add(1)
	go func() {
		defer a.reflectWG.Done()
		a.refreshInterests(ctx)
	}()

	return a.interestCache
}

func (a *Agent) refreshInterests(ctx context.Context) {
	defer func() {
		a.interestsMu.Lock()
		a.interestsRefreshing = false
		a.interestsMu.Unlock()
	}()

	rules := a.identity.Get().MemoryUsageRules

	minCluster := rules.MinClusterSize
	if minCluster <= 0 {
		minCluster = 2
	}

	clusters, err := a.memStore.FindClusters(ctx, minCluster)
	if err != nil {
		log.Printf("[agent] interests error: %v", err)
		return
	}

	log.Printf("[agent] interests recomputed: %d clusters", len(clusters))

	a.interestsMu.Lock()
	a.interestCache = clusters
	a.interestsAt = time.Now()
	a.interestsMu.Unlock()
}

func (a *Agent) latestReflection(ctx context.Context) memory.Node {
	a.lastReflectionMu.Lock()
	defer a.lastReflectionMu.Unlock()

	if !a.lastReflection.CreatedAt.IsZero() && time.Since(a.lastReflectionAt) < 5*time.Minute {
		return a.lastReflection
	}

	node, err := a.memStore.LatestedReflections(ctx)
	if err != nil {
		log.Printf("[agent] latest reflection error: %v", err)
		return a.lastReflection
	}

	if !node.CreatedAt.IsZero() {
		a.lastReflection = node
		a.lastReflectionAt = time.Now()
	}

	return node
}

func applyRecency(nodes []memory.Node, w float64) {
	newest, oldest := nodes[0].CreatedAt, nodes[0].CreatedAt
	for _, n := range nodes {
		if n.CreatedAt.After(newest) {
			newest = n.CreatedAt
		}
		if n.CreatedAt.Before(oldest) {
			oldest = n.CreatedAt
		}
	}

	span := newest.Sub(oldest)
	for i := range nodes {
		r := 1.0
		if span > 0 {
			r = nodes[i].CreatedAt.Sub(oldest).Seconds() / span.Seconds()
		}
		nodes[i].Similarity = (1-w)*nodes[i].Similarity + w*r
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Similarity > nodes[j].Similarity })
}

func interestLabel(cluster []memory.Node) string {
	best := cluster[0]
	for _, n := range cluster[1:] {
		if len(n.Content) < len(best.Content) {
			best = n
		}
	}

	if idx := strings.Index(best.Content, ":"); idx > 0 {
		return strings.TrimSpace(best.Content[:idx])
	}
	return best.Content
}
