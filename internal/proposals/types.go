package proposals

import "time"

type Proposal struct {
	ReflectionID string         `json:"reflection_id"`
	Timestamp    time.Time      `json:"timestamp"`
	Summary      string         `json:"conversation_summary"`
	Journal      *JournalProp   `json:"journal,omitempty"`
	Memory       *MemoryProp    `json:"memories,omitempty"`
	Identity     []IdentityProp `json:"identity,omitempty"`
}

type IdentityProp struct {
	Field   string `json:"field"`            // "values" | "conversational_principles"
	Action  string `json:"action"`           // "add" | "update"
	Target  string `json:"target,omitempty"` // existing item to reword (update only)
	Content string `json:"content"`
	Reason  string `json:"reason"`
}

type JournalProp struct {
	Content string `json:"content"`
	Mood    string `json:"mood"`
}

var Moods = map[string]string{
	"neutral":      "sin un tono emocional marcado",
	"tranquila":    "calma, conversación sin sobresaltos",
	"atenta":       "enfocada en entender o ayudar al usuario",
	"curiosa":      "interés por aprender o explorar algo nuevo",
	"contenta":     "alegría o bienestar general",
	"satisfecha":   "sensación de logro o de algo bien resuelto",
	"entusiasmada": "emoción fuerte por un logro o una novedad",
	"reflexiva":    "introspección, pensar en algo profundo",
	"preocupada":   "inquietud por algo que le pasa al usuario",
	"triste":       "pesar o melancolía",
	"frustrada":    "algo no salió como se esperaba",
}

const DefaultMood = "neutral"

type MemoryProp struct {
	Nodes []NodeProp `json:"nodes"`
	Edges []EdgeProp `json:"edges,omitempty"`
}

type EdgeProp struct {
	Type     string `json:"type"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relation string `json:"relation"`
}

type NodeProp struct {
	Type       string   `json:"type,omitempty"`
	Label      string   `json:"label"`
	Content    string   `json:"content"`
	Importance *float64 `json:"-"`
}
