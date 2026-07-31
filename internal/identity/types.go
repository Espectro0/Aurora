package identity

type IdentityCore struct {
	Name                     string
	Description              string
	Values                   []string
	Purpose                  string
	FoundationalMemories     []string
	ConversationalPrinciples []string
	MemoryUsageRules         MemoryRules
}

type MemoryRules struct {
	RecencyWeight              float64
	SemanticRelevanceThreshold float64
	MaxContextMemories         int
	ReflectionInterval         int
}
