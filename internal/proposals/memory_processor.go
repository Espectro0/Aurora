package proposals

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/google/uuid"
)

type MemoryProcessor struct {
	store     memory.MemoryStore
	journal   *SimpleProcessor
	threshold float64
}

func NewMemoryProcessor(store memory.MemoryStore, journalPath string, threshold float64) *MemoryProcessor {
	return &MemoryProcessor{
		store:     store,
		journal:   NewSimpleProcessor(journalPath),
		threshold: threshold,
	}
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

	if best != nil && best.Similarity >= p.threshold {
		log.Printf("[memory] reusing existing node %s (%s, %.2f)", best.ID, best.Content, best.Similarity)
		return *best, nil
	}

	node := memory.Node{
		ID:        uuid.New().String(),
		Type:      memory.NodeType(np.Type),
		Content:   label + ": " + np.Content,
		CreatedAt: ts,
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
