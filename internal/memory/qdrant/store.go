package qdrant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/Espectro0/AuroraProject/internal/embedder"
	"github.com/Espectro0/AuroraProject/internal/memory"
)

const defaultClusterThreshold = 0.70

type Config struct {
	BaseURL    string
	APIKey     string
	Collection string
	EdgesPath  string
}

type Store struct {
	mu               sync.RWMutex
	client           *restClient
	collection       string
	embedder         embedder.Embedder
	clusterThreshold float64
	edgesPath        string
	edges            map[string][]memory.Edge
}

func NewStore(cfg Config, e embedder.Embedder) (*Store, error) {
	if cfg.Collection == "" {
		cfg.Collection = "aurora_memories"
	}
	if cfg.EdgesPath == "" {
		return nil, fmt.Errorf("qdrant: edges path required")
	}

	warnIfInsecure(cfg.BaseURL, cfg.APIKey)

	client := newRESTClient(cfg.BaseURL, cfg.APIKey)
	ctx := context.Background()

	info, err := client.getCollection(ctx, cfg.Collection)
	if err != nil {
		return nil, fmt.Errorf("qdrant: get collection: %w", err)
	}
	if info == nil {
		vec, err := e.Embed(ctx, "aurora-init")
		if err != nil {
			return nil, fmt.Errorf("qdrant: init embed: %w", err)
		}
		if err := client.createCollection(ctx, cfg.Collection, len(vec)); err != nil {
			return nil, fmt.Errorf("qdrant: create collection: %w", err)
		}
		log.Printf("[qdrant] created collection %q (dim=%d)", cfg.Collection, len(vec))
	}

	s := &Store{
		client:           client,
		collection:       cfg.Collection,
		embedder:         e,
		clusterThreshold: defaultClusterThreshold,
		edgesPath:        cfg.EdgesPath,
		edges:            make(map[string][]memory.Edge),
	}

	if err := s.loadEdges(); err != nil {
		return nil, err
	}

	return s, nil
}

func warnIfInsecure(baseURL, apiKey string) {
	if apiKey != "" {
		return
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return
	}
	log.Printf("[qdrant] warning: connecting to %s without QDRANT_API_KEY set", baseURL)
}

func (s *Store) SetClusterThreshold(v float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if v > 0 {
		s.clusterThreshold = v
	}
}

func (s *Store) CreateNode(ctx context.Context, node memory.Node) error {
	vec, err := s.embedder.Embed(ctx, node.Content)
	if err != nil {
		return fmt.Errorf("qdrant: embed: %w", err)
	}

	p := point{ID: node.ID, Vector: vec, Payload: nodeToPayload(node)}
	if err := s.client.upsertPoint(ctx, s.collection, p); err != nil {
		return fmt.Errorf("qdrant: upsert: %w", err)
	}

	return nil
}

func (s *Store) GetNode(ctx context.Context, id string) (memory.Node, error) {
	points, err := s.client.retrievePoints(ctx, s.collection, []string{id})
	if err != nil {
		return memory.Node{}, fmt.Errorf("qdrant: retrieve: %w", err)
	}
	if len(points) == 0 {
		return memory.Node{}, fmt.Errorf("qdrant: node %s not found", id)
	}

	return pointToNode(points[0]), nil
}

func (s *Store) SearchNodes(ctx context.Context, query string, limit int) ([]memory.Node, error) {
	count, err := s.client.countPoints(ctx, s.collection)
	if err != nil {
		return nil, fmt.Errorf("qdrant: count: %w", err)
	}
	if count == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > count {
		limit = count
	}

	vec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("qdrant: embed: %w", err)
	}

	points, err := s.client.searchPoints(ctx, s.collection, vec, limit)
	if err != nil {
		return nil, fmt.Errorf("qdrant: search: %w", err)
	}

	nodes := make([]memory.Node, 0, len(points))
	for _, p := range points {
		nodes = append(nodes, pointToNode(p))
	}

	return nodes, nil
}

func (s *Store) CreateEdge(ctx context.Context, edge memory.Edge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.edges[edge.SourceID] {
		if existing.TargetID == edge.TargetID && existing.Type == edge.Type {
			return nil
		}
	}

	s.edges[edge.SourceID] = append(s.edges[edge.SourceID], edge)

	return s.saveEdges()
}

func (s *Store) GetEdges(ctx context.Context, nodeID string) ([]memory.Edge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]memory.Edge, len(s.edges[nodeID]))
	copy(result, s.edges[nodeID])

	return result, nil
}

func (s *Store) GetNeighbors(ctx context.Context, nodeID string, limit int) ([]memory.Node, error) {
	s.mu.RLock()
	seen := make(map[string]bool)
	ids := make([]string, 0, 8)
	for _, edges := range s.edges {
		for _, e := range edges {
			var other string
			if e.SourceID == nodeID {
				other = e.TargetID
			} else if e.TargetID == nodeID {
				other = e.SourceID
			}
			if other == "" || seen[other] {
				continue
			}
			seen[other] = true
			ids = append(ids, other)
		}
	}
	s.mu.RUnlock()

	if limit <= 0 || limit > len(ids) {
		limit = len(ids)
	}
	ids = ids[:limit]

	nodes := make([]memory.Node, 0, len(ids))
	for _, id := range ids {
		node, err := s.GetNode(ctx, id)
		if err != nil {
			continue
		}
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (s *Store) FindClusters(ctx context.Context, minClusterSize int) ([][]memory.Node, error) {
	count, err := s.client.countPoints(ctx, s.collection)
	if err != nil {
		return nil, fmt.Errorf("qdrant: count: %w", err)
	}
	if count == 0 {
		return nil, nil
	}

	filter := map[string]any{
		"must": []map[string]any{
			{"key": "type", "match": map[string]any{"value": string(memory.NodeConcept)}},
		},
	}

	points, err := s.client.scrollPoints(ctx, s.collection, filter)
	if err != nil {
		return nil, fmt.Errorf("qdrant: cluster scroll: %w", err)
	}

	if len(points) < minClusterSize {
		return nil, nil
	}

	assigned := make([]bool, len(points))
	var clusters [][]memory.Node

	for i := range points {
		if assigned[i] {
			continue
		}
		assigned[i] = true

		cluster := []memory.Node{pointToNode(points[i])}
		for j := range points {
			if i == j || assigned[j] {
				continue
			}
			if cosine(points[i].Vector, points[j].Vector) >= s.clusterThreshold {
				assigned[j] = true
				cluster = append(cluster, pointToNode(points[j]))
			}
		}

		if len(cluster) >= minClusterSize {
			clusters = append(clusters, cluster)
		}
	}

	return clusters, nil
}

func (s *Store) Count() int {
	count, err := s.client.countPoints(context.Background(), s.collection)
	if err != nil {
		log.Printf("[qdrant] count error: %v", err)
		return 0
	}
	return count
}

func (s *Store) Edges() []memory.Edge {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []memory.Edge
	for _, edges := range s.edges {
		result = append(result, edges...)
	}
	return result
}

func (s *Store) LatestedReflections(ctx context.Context) (memory.Node, error) {
	s.mu.RLock()
	sourceIDs := make([]string, 0, len(s.edges))
	for id := range s.edges {
		sourceIDs = append(sourceIDs, id)
	}
	s.mu.RUnlock()

	var best memory.Node
	found := false
	for _, id := range sourceIDs {
		node, err := s.GetNode(ctx, id)
		if err != nil {
			continue
		}
		if node.Type != memory.NodeReflection {
			continue
		}
		if !found || node.CreatedAt.After(best.CreatedAt) {
			best = node
			found = true
		}
	}

	return best, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveEdges()
}

func (s *Store) saveEdges() error {
	raw, err := json.MarshalIndent(s.edges, "", "  ")
	if err != nil {
		return fmt.Errorf("qdrant: marshal edges: %w", err)
	}

	return os.WriteFile(s.edgesPath, raw, 0644)
}

func (s *Store) loadEdges() error {
	raw, err := os.ReadFile(s.edgesPath)
	if err != nil {
		return nil
	}

	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}

	if err := json.Unmarshal(raw, &s.edges); err != nil {
		return fmt.Errorf("qdrant: unmarshal edges: %w", err)
	}

	return nil
}

func nodeToPayload(n memory.Node) map[string]any {
	payload := map[string]any{
		"type":       string(n.Type),
		"content":    n.Content,
		"created_at": n.CreatedAt.Format(time.RFC3339),
	}
	if n.Metadata != nil {
		payload["metadata"] = n.Metadata
	}
	return payload
}

func pointToNode(p point) memory.Node {
	n := memory.Node{ID: p.ID, Similarity: p.Score}

	if t, ok := p.Payload["type"].(string); ok {
		n.Type = memory.NodeType(t)
	}
	if c, ok := p.Payload["content"].(string); ok {
		n.Content = c
	}
	if m, ok := p.Payload["metadata"].(map[string]any); ok {
		n.Metadata = m
	}
	if ts, ok := p.Payload["created_at"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			n.CreatedAt = parsed
		}
	}

	return n
}

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		va := float64(a[i])
		vb := float64(b[i])
		dot += va * vb
		na += va * va
		nb += vb * vb
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
