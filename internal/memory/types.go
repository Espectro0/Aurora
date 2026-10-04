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

const (
	MetaImportance      = "importance"       // float64 in [0,1]
	MetaUpdatedAt       = "updated_at"       // RFC3339 string
	MetaPreviousContent = "previous_content" // content before the last replace
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

func (n Node) Importance() (float64, bool) {
	v, ok := n.Metadata[MetaImportance].(float64)
	return v, ok
}

func (n Node) LastTouched() time.Time {
	if s, ok := n.Metadata[MetaUpdatedAt].(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return n.CreatedAt
}
