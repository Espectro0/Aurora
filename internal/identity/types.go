package identity

type IdentityCore struct {
	Name                     string      `json:"name"`
	Description              string      `json:"description"`
	Values                   []string    `json:"values"`
	Purpose                  string      `json:"purpose"`
	FoundationalMemories     []string    `json:"foundational_memories"`
	ConversationalPrinciples []string    `json:"conversational_principles"`
	MemoryUsageRules         MemoryRules `json:"memory_usage_rules"`
	LLM                      LLMConfig   `json:"llm"`
}

type MemoryRules struct {
	RecencyWeight              float64 `json:"recency_weight"`
	SemanticRelevanceThreshold float64 `json:"semantic_relevance_threshold"`
	MaxContextMemories         int     `json:"max_context_memories"`
	ReflectionInterval         int     `json:"reflection_interval"`
	ReflectionHistory          int     `json:"reflection_history"`
	ClusterThreshold           float64 `json:"cluster_threshold"`
	MinClusterSize             int     `json:"min_cluster_size"`
	InterestTTLMinutes         int     `json:"interest_ttl_minutes"`
	ImportanceWeight           float64 `json:"importance_weight"`
	ReflectionGateThreshold    float64 `json:"reflection_gate_threshold"`
	WorthKeepingThreshold      float64 `json:"worth_keeping_threshold"`
	SameEntityThreshold        float64 `json:"same_entity_threshold"`
	NodeReplaceThreshold       float64 `json:"node_replace_threshold"`
	IdentityChangeThreshold    float64 `json:"identity_change_threshold"`
}

type LLMConfig struct {
	ChatTimeoutSeconds       int `json:"chat_timeout_seconds"`
	ReflectionTimeoutSeconds int `json:"reflection_timeout_seconds"`
	EmbedderTimeoutSeconds   int `json:"embedder_timeout_seconds"`
}
