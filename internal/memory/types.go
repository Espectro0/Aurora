package memory

import "time"

type NodeType string
type EdgeType string

const (
	NodePerson       NodeType = "person"
	NodeConversation NodeType = "conversation"
	NodeConcept      NodeType = "concept"
	NodeEvent        NodeType = "event"
	NodeReflection   NodeType = "reflection"
	NodeProject      NodeType = "project"
)

const (
	EdgeParticipates EdgeType = "participates"
	EdgeMentions     EdgeType = "mentions"
	EdgeRelates      EdgeType = "relates"
	EdgePrefers      EdgeType = "prefers"
	EdgeReflectsOn   EdgeType = "reflects_on"
	EdgeLeadsTo      EdgeType = "leads_to"
	EdgeSentiment    EdgeType = "sentiment"
)

type Node struct {
	ID         string
	Type       NodeType
	Content    string
	Metadata   map[string]any
	CreatedAt  time.Time
	Similarity float64
}

type Edge struct {
	ID        string
	SourceID  string
	TargetID  string
	Type      EdgeType
	Weight    float64
	CreatedAt time.Time
}
