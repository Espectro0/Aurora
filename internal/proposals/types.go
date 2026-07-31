package proposals

import "time"

type Proposal struct {
	ReflectionID string       `json:"reflection_id"`
	Timestamp    time.Time    `json:"timestamp"`
	Summary      string       `json:"conversation_summary"`
	Journal      *JournalProp `json:"journal,omitempty"`
}

type IdentityProp struct {
	Action  string `json:"action"` // "keep" | "update" | "add"
	Content string `json:"content"`
}

type JournalProp struct {
	Content string `json:"content"`
	Mood    string `json:"mood"`
}
