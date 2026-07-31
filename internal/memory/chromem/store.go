package chromem

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Espectro0/AuroraProject/internal/embedder"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/philippgille/chromem-go"
)

type Store struct {
	mu         sync.RWMutex
	path       string
	db         *chromem.DB
	collection *chromem.Collection
	edges      map[string][]memory.Edge
}

func NewStore(path string, e embedder.Embedder) (*Store, error) {
	db, err := chromem.NewPersistentDB(path+".vec", false)
	if err != nil {
		return nil, fmt.Errorf("chromem: db: %w", err)
	}

	embeddingFunc := func(ctx context.Context, text string) ([]float32, error) {
		return e.Embed(ctx, text)
	}

	collection := db.GetCollection("memories", embeddingFunc)
	if collection == nil {
		collection, err = db.CreateCollection("memories", nil, embeddingFunc)
		if err != nil {
			return nil, fmt.Errorf("chromem: collection: %w", err)
		}
	}

	s := &Store{
		path:       path,
		db:         db,
		collection: collection,
		edges:      make(map[string][]memory.Edge),
	}

	if err := s.loadEdges(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) CreateNode(ctx context.Context, node memory.Node) error {
	meta, err := nodeToMetadata(node)
	if err != nil {
		return fmt.Errorf("chromem: metadata: %w", err)
	}

	doc := chromem.Document{
		ID:       node.ID,
		Metadata: meta,
		Content:  node.Content,
	}

	if err := s.collection.AddDocument(ctx, doc); err != nil {
		return fmt.Errorf("chromem: add document: %w", err)
	}

	return nil
}

func (s *Store) GetNode(ctx context.Context, id string) (memory.Node, error) {
	doc, err := s.collection.GetByID(ctx, id)
	if err != nil {
		return memory.Node{}, fmt.Errorf("chromem: get by id: %w", err)
	}

	return docToNode(doc), nil
}

func (s *Store) SearchNodes(ctx context.Context, query string, limit int) ([]memory.Node, error) {
	count := s.collection.Count()
	if count == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > count {
		limit = count
	}

	results, err := s.collection.Query(ctx, query, limit, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("chromem: query: %w", err)
	}

	nodes := make([]memory.Node, 0, len(results))
	for _, r := range results {
		nodes = append(nodes, resultToNode(r))
	}

	return nodes, nil
}

func (s *Store) CreateEdge(ctx context.Context, edge memory.Edge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

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

func (s *Store) FindClusters(ctx context.Context, minClusterSize int) ([][]memory.Node, error) {
	return nil, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.saveEdges()
}

func (s *Store) saveEdges() error {
	raw, err := json.MarshalIndent(s.edges, "", "  ")
	if err != nil {
		return fmt.Errorf("chromem: marshal edges: %w", err)
	}

	return os.WriteFile(s.path+".edges.json", raw, 0644)
}

func (s *Store) loadEdges() error {
	raw, err := os.ReadFile(s.path + ".edges.json")
	if err != nil {
		return nil
	}

	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}

	if err := json.Unmarshal(raw, &s.edges); err != nil {
		return fmt.Errorf("chromem: unmarshal edges: %w", err)
	}

	return nil
}

func nodeToMetadata(n memory.Node) (map[string]string, error) {
	data, err := json.Marshal(n.Metadata)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"type":       string(n.Type),
		"created_at": n.CreatedAt.Format(time.RFC3339),
		"metadata":   string(data),
	}, nil
}

func docToNode(d chromem.Document) memory.Node {
	var metadata map[string]any
	if raw, ok := d.Metadata["metadata"]; ok && raw != "" {
		json.Unmarshal([]byte(raw), &metadata)
	}

	createdAt, _ := time.Parse(time.RFC3339, d.Metadata["created_at"])

	return memory.Node{
		ID:        d.ID,
		Type:      memory.NodeType(d.Metadata["type"]),
		Content:   d.Content,
		Metadata:  metadata,
		CreatedAt: createdAt,
	}
}

func resultToNode(r chromem.Result) memory.Node {
	var metadata map[string]any
	if raw, ok := r.Metadata["metadata"]; ok && raw != "" {
		json.Unmarshal([]byte(raw), &metadata)
	}

	createdAt, _ := time.Parse(time.RFC3339, r.Metadata["created_at"])

	return memory.Node{
		ID:         r.ID,
		Type:       memory.NodeType(r.Metadata["type"]),
		Content:    r.Content,
		Metadata:   metadata,
		CreatedAt:  createdAt,
		Similarity: float64(r.Similarity),
	}
}
