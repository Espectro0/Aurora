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
	NodeInterest     NodeType = "interest"
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
	MetaLastRecalledAt  = "last_recalled_at" // RFC3339 string, last time it was injected into a reply
	MetaRecallCount     = "recall_count"     // float64, times it was injected into a reply
	MetaTopic           = "topic"            // interest nodes: short topic name
	MetaReason          = "reason"           // interest nodes: why Aurora cares, first person
	MetaCuriosity       = "curiosity"        // interest nodes: something she'd like to explore
	MetaExploredAt      = "explored_at"      // interest nodes: RFC3339 string, last time it was researched while sleeping
	MetaSynthesizedAt   = "synthesized_at"   // RFC3339 string, last sleep that inferred connections from (or created) this node
)

type Node struct {
	ID         string
	Type       NodeType
	Content    string
	Owner      string
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

func (n Node) LastRecalled() time.Time {
	if s, ok := n.Metadata[MetaLastRecalledAt].(string); ok {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func (n Node) RecallCount() float64 {
	v, _ := n.Metadata[MetaRecallCount].(float64)
	return v
}

func (n Node) LastActive() time.Time {
	t := n.LastTouched()
	if r := n.LastRecalled(); r.After(t) {
		return r
	}
	return t
}
