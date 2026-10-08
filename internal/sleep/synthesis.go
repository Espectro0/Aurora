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
	"github.com/Espectro0/AuroraProject/internal/decision"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/google/uuid"
)

const (
	maxInferences            = 2
	maxSynthesisSamples      = 8
	maxSynthesizedImportance = 0.5
)

const (
	inferenceEdge    = "edge"
	inferenceConcept = "concept"
)

const synthesisPrompt = `Eres %s. %s
Estás durmiendo: repasas un grupo de recuerdos relacionados para descubrir conexiones que todavía no habías notado.

Te doy los recuerdos numerados. Propón como máximo %d inferencias que los conecten y que se desprendan razonablemente de ellos. No repitas lo que ya dice un recuerdo, no inventes datos nuevos y no fuerces conexiones: si no hay ninguna que valga la pena, devuelve la lista vacía.

Hay dos tipos de inferencia:
- "edge": una relación directa y duradera entre dos recuerdos existentes. "source" y "target" son números de recuerdo distintos. "type" es "relates" (relacionados temáticamente) o "leads_to" (uno conduce o lleva al otro). "relation" describe la relación en pocas palabras.
- "concept": una idea más general que agrupa varios recuerdos. "label" es corto y canónico (en minúscula si es un tema genérico). "content" es una sola frase de máximo 15 palabras, sin punto y coma (";"), que se entienda por sí sola. "links" son los números de los recuerdos que agrupa (al menos 2).

Responde SOLO con un JSON así, sin texto adicional:
{"inferences": [{"kind": "edge", "source": 0, "target": 2, "type": "leads_to", "relation": "..."}, {"kind": "concept", "label": "...", "content": "...", "links": [0, 1, 3]}]}`

type inference struct {
	Kind     string `json:"kind"`
	Source   int    `json:"source"`
	Target   int    `json:"target"`
	Type     string `json:"type"`
	Relation string `json:"relation"`
	Label    string `json:"label"`
	Content  string `json:"content"`
	Links    []int  `json:"links"`
}

var synthesisEdgeTypes = map[memory.EdgeType]bool{
	memory.EdgeRelates: true,
	memory.EdgeLeadsTo: true,
}

func (s *Sleeper) synthesize(ctx context.Context) ([]string, error) {
	id := s.identity.Get()
	cfg := id.Sleep
	if cfg.SynthesisDisabled || s.llm == nil {
		return nil, nil
	}

	clusters, err := s.store.FindClusters(ctx, id.MemoryUsageRules.MinClusterSize, memory.SharedOnly)
	if err != nil {
		return nil, fmt.Errorf("clusters: %w", err)
	}

	now := time.Now()
	cooldown := time.Duration(cfg.SynthesisCooldownDays) * 24 * time.Hour
	type candidate struct {
		scoredCluster
		sample []memory.Node
	}
	var scored []candidate
	for _, c := range clusters {
		if !needsSynthesis(c, now, cooldown) {
			continue
		}
		sort.Slice(c, func(i, j int) bool { return importanceOf(c[i]) > importanceOf(c[j]) })
		sample := c
		if len(sample) > maxSynthesisSamples {
			sample = sample[:maxSynthesisSamples]
		}
		var strength float64
		for _, n := range c {
			strength += importanceOf(n)
		}
		scored = append(scored, candidate{scoredCluster{nodes: c, strength: strength}, sample})
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].strength > scored[j].strength })
	if len(scored) > cfg.MaxSyntheses {
		scored = scored[:cfg.MaxSyntheses]
	}

	var made []string
	for _, c := range scored {
		if ctx.Err() != nil {
			break
		}
		inferences, err := s.proposeInferences(ctx, c.sample)
		if err != nil {
			log.Printf("[sleep] synthesis: %v", err)
			continue
		}
		for _, inf := range inferences {
			if desc, ok := s.applyInference(ctx, c.sample, inf, now); ok {
				made = append(made, desc)
			}
		}
		s.markSynthesized(ctx, c.nodes, now)
	}
	return made, nil
}

func needsSynthesis(cluster []memory.Node, now time.Time, cooldown time.Duration) bool {
	for _, n := range cluster {
		at, _ := n.Metadata[memory.MetaSynthesizedAt].(string)
		t, err := time.Parse(time.RFC3339, at)
		if err != nil || now.Sub(t) >= cooldown {
			return true
		}
	}
	return false
}

func (s *Sleeper) markSynthesized(ctx context.Context, nodes []memory.Node, at time.Time) {
	for _, n := range nodes {
		meta := make(map[string]any, len(n.Metadata)+1)
		for k, v := range n.Metadata {
			meta[k] = v
		}
		meta[memory.MetaSynthesizedAt] = at.Format(time.RFC3339)
		if err := s.store.SetMetadata(ctx, n.ID, meta); err != nil {
			log.Printf("[sleep] mark synthesized %s: %v", n.ID, err)
		}
	}
}

func listMemories(nodes []memory.Node) string {
	var b strings.Builder
	for i, n := range nodes {
		fmt.Fprintf(&b, "%d. %s\n", i, n.Content)
	}
	return b.String()
}

func (s *Sleeper) proposeInferences(ctx context.Context, nodes []memory.Node) ([]inference, error) {
	id := s.identity.Get()
	msgs := []conversation.Message{
		conversation.NewMessage(conversation.System, fmt.Sprintf(synthesisPrompt, id.Name, id.Description, maxInferences)),
		conversation.NewMessage(conversation.User, listMemories(nodes)),
	}

	raw, err := s.llm.Chat(ctx, msgs)
	if err != nil {
		return nil, fmt.Errorf("chat: %w", err)
	}
	out, err := parseInferences(raw)
	if err != nil {
		msgs = append(msgs,
			conversation.NewMessage(conversation.Assistant, raw),
			conversation.NewMessage(conversation.System, "Tu respuesta anterior no fue JSON valido. Responde SOLO con el JSON, sin texto adicional."))
		if raw, err = s.llm.Chat(ctx, msgs); err != nil {
			return nil, fmt.Errorf("chat retry: %w", err)
		}
		if out, err = parseInferences(raw); err != nil {
			return nil, err
		}
	}
	if len(out) > maxInferences {
		out = out[:maxInferences]
	}
	return out, nil
}

func parseInferences(raw string) ([]inference, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object in response")
	}
	var out struct {
		Inferences []inference `json:"inferences"`
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &out); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return out.Inferences, nil
}

func (s *Sleeper) applyInference(ctx context.Context, nodes []memory.Node, inf inference, now time.Time) (string, bool) {
	valid := func(i int) bool { return i >= 0 && i < len(nodes) }

	switch inf.Kind {
	case inferenceEdge:
		edgeType := memory.EdgeType(inf.Type)
		if !synthesisEdgeTypes[edgeType] {
			edgeType = memory.EdgeRelates
		}
		if !valid(inf.Source) || !valid(inf.Target) || inf.Source == inf.Target || strings.TrimSpace(inf.Relation) == "" {
			return "", false
		}
		src, dst := nodes[inf.Source], nodes[inf.Target]
		if related, err := s.related(ctx, src.ID, dst.ID); err != nil || related {
			return "", false
		}
		claim := fmt.Sprintf("%s → %s: %s", nodeLabel(src), nodeLabel(dst), inf.Relation)
		if !s.grounded(ctx, nodes, claim) {
			return "", false
		}
		if err := s.store.CreateEdge(ctx, memory.Edge{
			ID:        uuid.New().String(),
			SourceID:  src.ID,
			TargetID:  dst.ID,
			Type:      edgeType,
			Weight:    1,
			CreatedAt: now,
		}); err != nil {
			log.Printf("[sleep] synthesis edge: %v", err)
			return "", false
		}
		log.Printf("[sleep] inferred %s edge %s", edgeType, claim)
		return claim, true

	case inferenceConcept:
		label := strings.TrimSpace(inf.Label)
		content := strings.TrimSpace(strings.ReplaceAll(inf.Content, ";", ","))
		var linked []memory.Node
		seen := map[int]bool{}
		for _, i := range inf.Links {
			if valid(i) && !seen[i] {
				seen[i] = true
				linked = append(linked, nodes[i])
			}
		}
		if label == "" || content == "" || len(linked) < 2 {
			return "", false
		}
		full := label + ": " + content
		if !s.grounded(ctx, nodes, full) {
			return "", false
		}
		concept, created, err := s.ensureConcept(ctx, full, linked, now)
		if err != nil {
			log.Printf("[sleep] synthesis concept %q: %v", label, err)
			return "", false
		}
		links := 0
		for _, n := range linked {
			if n.ID == concept.ID {
				continue
			}
			if related, err := s.related(ctx, concept.ID, n.ID); err != nil || related {
				continue
			}
			if err := s.store.CreateEdge(ctx, memory.Edge{
				ID:        uuid.New().String(),
				SourceID:  concept.ID,
				TargetID:  n.ID,
				Type:      memory.EdgeRelates,
				Weight:    1,
				CreatedAt: now,
			}); err != nil {
				log.Printf("[sleep] synthesis concept edge: %v", err)
				continue
			}
			links++
		}
		if !created && links == 0 {
			return "", false
		}
		log.Printf("[sleep] inferred concept %q (new=%v, links=%d)", full, created, links)
		return label, true
	}
	return "", false
}

func (s *Sleeper) grounded(ctx context.Context, nodes []memory.Node, claim string) bool {
	if s.decision == nil {
		return true
	}
	threshold := s.identity.Get().Sleep.SynthesisThreshold
	answers, err := s.decision.Judge(ctx, fmt.Sprintf("Recuerdos:\n%s\nInferencia: %s", listMemories(nodes), claim), map[string]decision.Question{
		"grounded": {
			Type:         decision.TypeNoul,
			Instructions: "¿Esta inferencia se desprende razonablemente de los recuerdos, aporta una conexión nueva y no es inventada, forzada ni trivial?",
		},
	})
	if err != nil {
		log.Printf("[sleep] synthesis decision error, discarding %q: %v", claim, err)
		return false
	}
	ans, ok := answers["grounded"]
	if !ok || ans.Noul < threshold {
		log.Printf("[sleep] discarding ungrounded inference %q", claim)
		return false
	}
	return true
}

func (s *Sleeper) ensureConcept(ctx context.Context, content string, linked []memory.Node, now time.Time) (memory.Node, bool, error) {
	threshold := s.identity.Get().MemoryUsageRules.SemanticRelevanceThreshold
	results, err := s.store.SearchNodes(ctx, content, 5, memory.SharedOnly)
	if err != nil {
		return memory.Node{}, false, fmt.Errorf("search: %w", err)
	}
	for _, n := range results {
		if n.Type == memory.NodeConcept && n.Similarity >= threshold {
			return n, false, nil
		}
	}

	var sum float64
	for _, n := range linked {
		sum += importanceOf(n)
	}
	imp := math.Min(sum/float64(len(linked)), maxSynthesizedImportance)

	node := memory.Node{
		ID:        uuid.New().String(),
		Type:      memory.NodeConcept,
		Content:   content,
		CreatedAt: now,
		Metadata: map[string]any{
			memory.MetaImportance:    math.Round(imp*1000) / 1000,
			memory.MetaSynthesizedAt: now.Format(time.RFC3339),
		},
	}
	if err := s.store.CreateNode(ctx, node); err != nil {
		return memory.Node{}, false, err
	}
	return node, true, nil
}

func (s *Sleeper) related(ctx context.Context, a, b string) (bool, error) {
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		edges, err := s.store.GetEdges(ctx, pair[0])
		if err != nil {
			return false, err
		}
		for _, e := range edges {
			if e.TargetID == pair[1] && e.Type != memory.EdgeReflectsOn {
				return true, nil
			}
		}
	}
	return false, nil
}

func nodeLabel(n memory.Node) string {
	if idx := strings.Index(n.Content, ":"); idx > 0 {
		return strings.TrimSpace(n.Content[:idx])
	}
	return n.Content
}
