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
	GetNode(ctx context.Context, id string) (Node, error)
	SearchNodes(ctx context.Context, query string, limit int) ([]Node, error)
	CreateEdge(ctx context.Context, edge Edge) error
	GetEdges(ctx context.Context, nodeID string) ([]Edge, error)
	FindClusters(ctx context.Context, minClusterSize int) ([][]Node, error)
	Close() error
}
