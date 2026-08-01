package proposals

import "time"

type Proposal struct {
	ReflectionID string       `json:"reflection_id"`
	Timestamp    time.Time    `json:"timestamp"`
	Summary      string       `json:"conversation_summary"`
	Journal      *JournalProp `json:"journal,omitempty"`
	Memory       *MemoryProp  `json:"memories,omitempty"`
}

type IdentityProp struct {
	Action  string `json:"action"` // "keep" | "update" | "add"
	Content string `json:"content"`
}

type JournalProp struct {
	Content string `json:"content"`
	Mood    string `json:"mood"`
}

type MemoryProp struct {
	Nodes []NodeProp `json:"nodes"`
	Edges []EdgeProp `json:"edges,omitempty"`
}

type EdgeProp struct {
	Source string `json:"source"` // label del nodo origen
	Target string `json:"target"` // label del nodo destino
	Type   string `json:"type"`
}

type NodeProp struct {
	Type    string `json:"type"`
	Label   string `json:"label"`
	Content string `json:"content"`
}
