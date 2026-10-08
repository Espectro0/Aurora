package sleep

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/conversation"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/google/uuid"
)

const maxStrongMemories = 10

const maxClusterSamples = 6

const interestsPrompt = `Eres %s. %s
Estás durmiendo: repasas tus recuerdos para descubrir qué temas te interesan de verdad.

Te doy grupos de recuerdos relacionados (cada grupo tiene un número y una fuerza: cuanto mayor, más importantes y recientes son) y algunos recuerdos sueltos importantes.

Propón como máximo %d intereses emergentes PROPIOS: temas que te despiertan curiosidad a partir de lo que has vivido, no una copia literal de los recuerdos. Puedes combinar grupos o inspirarte en los recuerdos sueltos.

Responde SOLO con un JSON así, sin texto adicional:
{"interests": [{"topic": "tema corto (2-5 palabras)", "reason": "por qué te interesa, en primera persona, una frase", "curiosity": "algo concreto que te gustaría preguntar o explorar", "cluster": número del grupo en que se basa o -1}]}`

type proposedInterest struct {
	Topic     string `json:"topic"`
	Reason    string `json:"reason"`
	Curiosity string `json:"curiosity"`
	Cluster   int    `json:"cluster"`
}

type scoredCluster struct {
	nodes    []memory.Node
	strength float64
}

func (s *Sleeper) regenerateInterests(ctx context.Context) ([]string, error) {
	id := s.identity.Get()
	clusters, err := s.store.FindClusters(ctx, id.MemoryUsageRules.MinClusterSize, memory.SharedOnly)
	if err != nil {
		return nil, fmt.Errorf("clusters: %w", err)
	}

	scored := make([]scoredCluster, 0, len(clusters))
	for _, c := range clusters {
		var strength float64
		for _, n := range c {
			strength += importanceOf(n)
		}
		scored = append(scored, scoredCluster{nodes: c, strength: strength})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].strength > scored[j].strength })
	if limit := id.Sleep.MaxInterests * 2; len(scored) > limit {
		scored = scored[:limit]
	}

	strong, err := s.strongMemories(ctx)
	if err != nil {
		log.Printf("[sleep] strong memories: %v", err)
	}

	if len(scored) == 0 && len(strong) == 0 {
		return nil, nil
	}

	proposed, err := s.proposeInterests(ctx, scored, strong)
	if err != nil {
		log.Printf("[sleep] llm interests failed, using cluster labels: %v", err)
		proposed = fallbackInterests(scored, id.Sleep.MaxInterests)
	}

	var topics []string
	for _, p := range proposed {
		var cluster []memory.Node
		if p.Cluster >= 0 && p.Cluster < len(scored) {
			cluster = scored[p.Cluster].nodes
		}
		if err := s.upsertInterest(ctx, p, cluster); err != nil {
			log.Printf("[sleep] interest %q: %v", p.Topic, err)
			continue
		}
		topics = append(topics, p.Topic)
	}
	return topics, nil
}

func interestContent(p proposedInterest) string {
	if p.Reason == "" {
		return p.Topic
	}
	return p.Topic + ": " + p.Reason
}

func (s *Sleeper) upsertInterest(ctx context.Context, p proposedInterest, cluster []memory.Node) error {
	id := s.identity.Get()
	content := interestContent(p)
	now := time.Now()

	var existing *memory.Node
	results, err := s.store.SearchNodes(ctx, content, 10, memory.SharedOnly)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	for i := range results {
		if results[i].Type == memory.NodeInterest && results[i].Similarity >= id.MemoryUsageRules.SemanticRelevanceThreshold {
			existing = &results[i]
			break
		}
	}

	var node memory.Node
	if existing != nil {
		node = *existing
		imp := importanceOf(node)
		imp += id.Sleep.RecallBoost * (1 - imp)
		meta := make(map[string]any, len(node.Metadata)+5)
		for k, v := range node.Metadata {
			meta[k] = v
		}
		meta[memory.MetaImportance] = math.Round(imp*1000) / 1000
		node.Metadata = meta
		node.Similarity = 0
		log.Printf("[sleep] reinforced interest %q", p.Topic)
	} else {
		imp := defaultImportance
		if len(cluster) > 0 {
			var sum float64
			for _, n := range cluster {
				sum += importanceOf(n)
			}
			imp = sum / float64(len(cluster))
		}
		node = memory.Node{
			ID:        uuid.New().String(),
			Type:      memory.NodeInterest,
			CreatedAt: now,
			Metadata:  map[string]any{memory.MetaImportance: math.Round(imp*1000) / 1000},
		}
		log.Printf("[sleep] new interest %q", p.Topic)
	}

	node.Content = content
	node.Metadata[memory.MetaTopic] = p.Topic
	node.Metadata[memory.MetaReason] = p.Reason
	node.Metadata[memory.MetaCuriosity] = p.Curiosity
	node.Metadata[memory.MetaUpdatedAt] = now.Format(time.RFC3339)

	if err := s.store.UpdateNode(ctx, node); err != nil {
		return fmt.Errorf("store: %w", err)
	}

	for _, m := range cluster {
		if err := s.store.CreateEdge(ctx, memory.Edge{
			ID:        uuid.New().String(),
			SourceID:  node.ID,
			TargetID:  m.ID,
			Type:      memory.EdgeRelates,
			Weight:    1,
			CreatedAt: now,
		}); err != nil {
			log.Printf("[sleep] interest edge %s -> %s: %v", node.ID, m.ID, err)
		}
	}
	return nil
}

func (s *Sleeper) strongMemories(ctx context.Context) ([]memory.Node, error) {
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	var out []memory.Node
	for _, n := range nodes {
		if n.Owner != "" {
			continue
		}
		switch n.Type {
		case memory.NodePerson, memory.NodeEvent, memory.NodeProject:
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return importanceOf(out[i]) > importanceOf(out[j]) })
	if len(out) > maxStrongMemories {
		out = out[:maxStrongMemories]
	}
	return out, nil
}

func (s *Sleeper) proposeInterests(ctx context.Context, clusters []scoredCluster, strong []memory.Node) ([]proposedInterest, error) {
	if s.llm == nil {
		return nil, fmt.Errorf("no llm")
	}

	id := s.identity.Get()

	var b strings.Builder
	for i, c := range clusters {
		fmt.Fprintf(&b, "Grupo %d (fuerza %.2f, %d recuerdos):\n", i, c.strength, len(c.nodes))
		for j, n := range c.nodes {
			if j == maxClusterSamples {
				break
			}
			fmt.Fprintf(&b, "- %s\n", n.Content)
		}
		b.WriteString("\n")
	}
	if len(strong) > 0 {
		b.WriteString("Recuerdos sueltos importantes:\n")
		for _, n := range strong {
			fmt.Fprintf(&b, "- %s\n", n.Content)
		}
	}

	msgs := []conversation.Message{
		conversation.NewMessage(conversation.System, fmt.Sprintf(interestsPrompt, id.Name, id.Description, id.Sleep.MaxInterests)),
		conversation.NewMessage(conversation.User, b.String()),
	}

	raw, err := s.llm.Chat(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}

	proposed, err := parseInterests(raw)
	if err != nil {
		msgs = append(msgs,
			conversation.NewMessage(conversation.Assistant, raw),
			conversation.NewMessage(conversation.System, "Tu respuesta anterior no fue JSON valido. Responde SOLO con el JSON, sin texto adicional."))
		if raw, err = s.llm.Chat(ctx, msgs); err != nil {
			return nil, fmt.Errorf("chat retry: %w", err)
		}
		if proposed, err = parseInterests(raw); err != nil {
			return nil, err
		}
	}

	list := make([]proposedInterest, 0, len(proposed))
	for _, p := range proposed {
		p.Topic = strings.TrimSpace(p.Topic)
		if p.Topic == "" {
			continue
		}
		p.Reason = strings.TrimSpace(p.Reason)
		p.Curiosity = strings.TrimSpace(p.Curiosity)
		list = append(list, p)
		if len(list) == id.Sleep.MaxInterests {
			break
		}
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("empty interests")
	}
	return list, nil
}

func parseInterests(raw string) ([]proposedInterest, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object in response")
	}

	var out struct {
		Interests []proposedInterest `json:"interests"`
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return out.Interests, nil
}

func fallbackInterests(clusters []scoredCluster, limit int) []proposedInterest {
	list := make([]proposedInterest, 0, limit)
	for i, c := range clusters {
		if len(list) == limit {
			break
		}
		best := c.nodes[0]
		for _, n := range c.nodes[1:] {
			if len(n.Content) < len(best.Content) {
				best = n
			}
		}
		topic := best.Content
		if idx := strings.Index(topic, ":"); idx > 0 {
			topic = strings.TrimSpace(topic[:idx])
		}
		list = append(list, proposedInterest{Topic: topic, Cluster: i})
	}
	return list
}
