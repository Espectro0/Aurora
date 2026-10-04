package proposals

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/decision"
	"github.com/Espectro0/AuroraProject/internal/identity"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/google/uuid"
)

type MemoryProcessor struct {
	store    memory.MemoryStore
	journal  *SimpleProcessor
	decision decision.Provider
	identity *identity.Core
	cfg      Config
}

type Config struct {
	JournalPath             string
	SimilarityThreshold     float64
	SameEntityThreshold     float64
	NodeReplaceThreshold    float64
	IdentityChangeThreshold float64
}

func NewMemoryProcessor(store memory.MemoryStore, dec decision.Provider, id *identity.Core, cfg Config) *MemoryProcessor {
	return &MemoryProcessor{
		store:    store,
		decision: dec,
		identity: id,
		journal:  NewSimpleProcessor(cfg.JournalPath),
		cfg:      cfg,
	}
}

func (p *MemoryProcessor) Journal() *SimpleProcessor {
	return p.journal
}

func (p *MemoryProcessor) Process(ctx context.Context, prop Proposal) error {
	if err := p.journal.Process(ctx, prop); err != nil {
		return err
	}

	reflectionNode := memory.Node{
		ID:        prop.ReflectionID,
		Type:      memory.NodeReflection,
		Content:   prop.Summary,
		Metadata:  map[string]any{"reflection_id": prop.ReflectionID},
		CreatedAt: prop.Timestamp,
	}

	if err := p.store.CreateNode(ctx, reflectionNode); err != nil {
		return fmt.Errorf("memory: reflection node: %w", err)
	}

	p.processIdentity(ctx, prop)

	if prop.Memory == nil {
		return nil
	}

	nodes := prop.Memory.Nodes
	seen := make(map[string]bool, len(nodes))
	dedup := nodes[:0]
	for _, np := range nodes {
		if l := normalizeLabel(np.Type, np.Label); !seen[l] {
			seen[l] = true
			dedup = append(dedup, np)
		}
	}

	idByLabel := make(map[string]string, len(dedup))
	for _, np := range dedup {
		target, err := p.ensureNode(ctx, np, prop.Timestamp)
		if err != nil {
			return err
		}

		idByLabel[normalizeLabel(np.Type, np.Label)] = target.ID

		if err := p.store.CreateEdge(ctx, memory.Edge{
			ID:        uuid.New().String(),
			SourceID:  reflectionNode.ID,
			TargetID:  target.ID,
			Type:      memory.EdgeReflectsOn,
			Weight:    1,
			CreatedAt: prop.Timestamp,
		}); err != nil {
			return fmt.Errorf("memory: edge: %w", err)
		}
	}

	for _, ep := range prop.Memory.Edges {
		if err := p.createEntityEdge(ctx, ep, idByLabel, prop.Timestamp); err != nil {
			return err
		}
	}

	return nil
}

func (p *MemoryProcessor) createEntityEdge(ctx context.Context, ep EdgeProp, idByLabel map[string]string, ts time.Time) error {
	if ep.Source == "" || ep.Target == "" || ep.Source == ep.Target {
		return nil
	}

	edgeType := memory.EdgeType(ep.Type)
	if !validEdgeType(edgeType) {
		log.Printf("[memory] skipping edge with unknown type %q", ep.Type)
		return nil
	}

	sourceID, ok := resolveEdgeLabel(ep.Source, idByLabel)
	if !ok {
		log.Printf("[memory] skipping edge, source %q not in nodes", ep.Source)
		return nil
	}
	targetID, ok := resolveEdgeLabel(ep.Target, idByLabel)
	if !ok {
		log.Printf("[memory] skipping edge, target %q not in nodes", ep.Target)
		return nil
	}
	if sourceID == targetID {
		return nil
	}

	exists, err := p.relationExists(ctx, sourceID, targetID)
	if err != nil {
		log.Printf("[memory] edge lookup error, creating anyway: %v", err)
	} else if exists {
		log.Printf("[memory] skipping edge, %s and %s are already related", ep.Source, ep.Target)
		return nil
	}

	if err := p.store.CreateEdge(ctx, memory.Edge{
		ID:        uuid.New().String(),
		SourceID:  sourceID,
		TargetID:  targetID,
		Type:      edgeType,
		Weight:    1,
		CreatedAt: ts,
	}); err != nil {
		return fmt.Errorf("memory: edge: %w", err)
	}

	log.Printf("[memory] created %s edge %s -> %s", ep.Type, ep.Source, ep.Target)
	return nil
}

func validEdgeType(t memory.EdgeType) bool {
	switch t {
	case memory.EdgeParticipates, memory.EdgeMentions, memory.EdgeRelates,
		memory.EdgePrefers, memory.EdgeReflectsOn, memory.EdgeLeadsTo, memory.EdgeSentiment:
		return true
	default:
		return false
	}
}

func normalizeEdgeLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

func resolveEdgeLabel(label string, idByLabel map[string]string) (string, bool) {
	if id, ok := idByLabel[strings.TrimSpace(label)]; ok {
		return id, true
	}
	id, ok := idByLabel[normalizeEdgeLabel(label)]
	return id, ok
}

func (p *MemoryProcessor) ensureNode(ctx context.Context, np NodeProp, ts time.Time) (memory.Node, error) {
	label := normalizeLabel(np.Type, np.Label)

	results, err := p.store.SearchNodes(ctx, label, 5)
	if err != nil {
		return memory.Node{}, fmt.Errorf("memory: search: %w", err)
	}

	var best *memory.Node
	targetType := memory.NodeType(np.Type)
	for i := range results {
		n := &results[i]
		if n.Type != targetType {
			continue
		}
		if best == nil || n.Similarity > best.Similarity {
			best = n
		}
	}

	if best != nil {
		verdict, err := p.compareNode(ctx, np, *best)
		if err != nil {
			log.Printf("[memory] decision error, falling back to cosine: %v", err)
			verdict = nodeDifferent
			if best.Similarity >= p.cfg.SimilarityThreshold {
				verdict = nodeSame
			}
		}

		switch verdict {
		case nodeSame:
			log.Printf("[memory] reusing existing node %s (%s)", best.ID, best.Content)
			return *best, nil
		case nodeComplements, nodeReplaces:
			updated := mergeNode(*best, label, np, verdict, ts)
			if err := p.store.UpdateNode(ctx, updated); err != nil {
				return memory.Node{}, fmt.Errorf("memory: update node: %w", err)
			}
			log.Printf("[memory] %s node %s: %q", verdict, best.ID, updated.Content)
			return updated, nil
		}
	}

	node := memory.Node{
		ID:        uuid.New().String(),
		Type:      memory.NodeType(np.Type),
		Content:   label + ": " + np.Content,
		CreatedAt: ts,
	}
	if np.Importance != nil {
		node.Metadata = map[string]any{memory.MetaImportance: *np.Importance}
	}

	if err := p.store.CreateNode(ctx, node); err != nil {
		return memory.Node{}, fmt.Errorf("memory: node: %w", err)
	}

	log.Printf("[memory] created %s node %q", np.Type, label)
	return node, nil
}

func normalizeLabel(nodeType, label string) string {
	label = strings.TrimSpace(label)
	if nodeType == string(memory.NodeConcept) || nodeType == string(memory.NodeEvent) {
		return strings.ToLower(label)
	}
	return label
}

const (
	nodeSame        = "same"
	nodeComplements = "complements"
	nodeReplaces    = "replaces"
	nodeDifferent   = "different"
)

var nodeVerdicts = map[string]string{
	nodeSame:        "es la misma entidad y el mismo hecho; no aporta nada nuevo",
	nodeComplements: "es la misma entidad y agrega un hecho nuevo compatible con lo que ya se sabe",
	nodeReplaces:    "es la misma entidad pero el hecho nuevo contradice o deja obsoleto lo que ya se sabe",
	nodeDifferent:   "es una entidad distinta",
}

const maxFactsPerNode = 3

func (p *MemoryProcessor) compareNode(ctx context.Context, np NodeProp, existing memory.Node) (string, error) {
	if p.decision == nil {
		if existing.Similarity >= p.cfg.SimilarityThreshold {
			return nodeSame, nil
		}
		return nodeDifferent, nil
	}

	state := fmt.Sprintf("Recuerdo existente: %s\nInformación nueva: %s: %s", existing.Content, np.Label, np.Content)
	answers, err := p.decision.Judge(ctx, state, map[string]decision.Question{
		"relation": {
			Type:         decision.TypeChoice,
			Instructions: "¿Cómo se relaciona la información nueva con el recuerdo existente?",
			Criteria:     nodeVerdicts,
		},
	})
	if err != nil {
		return "", fmt.Errorf("decision: relation: %w", err)
	}
	ans, ok := answers["relation"]
	if !ok {
		return "", fmt.Errorf("decision: relation: missing answer")
	}
	if _, ok := nodeVerdicts[ans.Choice]; !ok {
		return "", fmt.Errorf("decision: relation: invalid choice %q", ans.Choice)
	}

	pSame, pChoice := 1.0, ans.Confidence
	if len(ans.Probabilities) > 0 {
		pSame = 1 - ans.Probabilities[nodeDifferent]
		pChoice = ans.Probabilities[ans.Choice]
	}

	verdict := ans.Choice
	if verdict != nodeDifferent && pSame < p.cfg.SameEntityThreshold {
		verdict = nodeDifferent
	}
	if verdict == nodeReplaces && pChoice < p.cfg.NodeReplaceThreshold {
		verdict = nodeComplements
	}
	return verdict, nil
}

func mergeNode(existing memory.Node, label string, np NodeProp, verdict string, ts time.Time) memory.Node {
	prefix, facts := label, []string(nil)
	if idx := strings.Index(existing.Content, ": "); idx > 0 {
		prefix = existing.Content[:idx]
		facts = strings.Split(existing.Content[idx+2:], "; ")
	}

	meta := make(map[string]any, len(existing.Metadata)+3)
	for k, v := range existing.Metadata {
		meta[k] = v
	}
	meta[memory.MetaUpdatedAt] = ts.Format(time.RFC3339)
	if np.Importance != nil {
		if old, ok := existing.Importance(); !ok || *np.Importance > old {
			meta[memory.MetaImportance] = *np.Importance
		}
	}

	switch verdict {
	case nodeReplaces:
		meta[memory.MetaPreviousContent] = existing.Content
		facts = []string{np.Content}
	default:
		facts = append(facts, np.Content)
		if len(facts) > maxFactsPerNode {
			facts = facts[len(facts)-maxFactsPerNode:]
		}
	}

	existing.Content = prefix + ": " + strings.Join(facts, "; ")
	existing.Metadata = meta
	existing.Similarity = 0
	return existing
}

func (p *MemoryProcessor) relationExists(ctx context.Context, a, b string) (bool, error) {
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		edges, err := p.store.GetEdges(ctx, pair[0])
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
