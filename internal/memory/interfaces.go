package memory

import (
	"context"

	"github.com/Espectro0/AuroraProject/internal/conversation"
)

type Store interface {
	Save(userID string, message conversation.Message) error
	History(userId string) []conversation.Message
}

type MemoryStore interface {
	CreateNode(ctx context.Context, node Node) error
	UpdateNode(ctx context.Context, node Node) error
	GetNode(ctx context.Context, id string) (Node, error)
	SearchNodes(ctx context.Context, query string, limit int, scope Scope) ([]Node, error)
	CreateEdge(ctx context.Context, edge Edge) error
	GetEdges(ctx context.Context, nodeID string) ([]Edge, error)
	EdgesJSON() ([]byte, error)
	GetNeighbors(ctx context.Context, nodeID string, limit int) ([]Node, error)
	FindClusters(ctx context.Context, minClusterSize int, scope Scope) ([][]Node, error)
	LatestedReflections(ctx context.Context, scope Scope) (Node, error)
	ListNodes(ctx context.Context) ([]Node, error)
	NodesByType(ctx context.Context, t NodeType) ([]Node, error)
	FindDuplicates(ctx context.Context, threshold float64) ([][]Node, error)
	SetMetadata(ctx context.Context, id string, metadata map[string]any) error
	SetOwner(ctx context.Context, id string, owner string) error
	DeleteNode(ctx context.Context, id string) error
	RewireEdges(ctx context.Context, fromID, toID string) error
	Count() int
	Edges() []Edge
	Close() error
}
