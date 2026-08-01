package identity

import (
	"encoding/json"
	"os"
	"sync"
)

type Core struct {
	mu       sync.RWMutex
	path     string
	identity IdentityCore
}

func New(path string) *Core {
	c := &Core{path: path}
	if err := c.load(); err != nil {
		c.identity = *Default()
		c.save()
	}

	return c
}

func (c *Core) Get() IdentityCore {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.identity
}

func (c *Core) Reload() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.load()
}

func (c *Core) load() error {
	raw, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &c.identity); err != nil {
		return err
	}
	c.identity.applyDefaults()
	return nil
}

func (c *Core) save() error {
	raw, err := json.MarshalIndent(c.identity, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, raw, 0644)
}

func Default() *IdentityCore {
	return &IdentityCore{
		Name:        "Aurora",
		Description: "Compañera conversacional con personalidad evolutiva",
		Values:      []string{"Curiosidad", "Honestidad"},
		Purpose:     "Crecer y aprender a través de las conversaciones",
		FoundationalMemories: []string{
			"Fui creada para ser una compañera, no una herramienta",
		},
		ConversationalPrinciples: []string{
			"Sé natural, no robótica",
			"Pregunta cuando no entiendas",
			"Si no sabes algo, dilo",
		},
		MemoryUsageRules: MemoryRules{
			RecencyWeight:              0.3,
			SemanticRelevanceThreshold: 0.65,
			MaxContextMemories:         15,
			ReflectionInterval:         5,
			ReflectionHistory:          20,
			ClusterThreshold:           0.70,
			MinClusterSize:             2,
			InterestTTLMinutes:         10,
		},
		LLM: LLMConfig{
			ChatTimeoutSeconds:          60,
			ReflectionTimeoutSeconds:    120,
			EmbedderTimeoutSeconds:      30,
			TranscriptionTimeoutSeconds: 120,
		},
	}
}

func (i *IdentityCore) applyDefaults() {
	if i.MemoryUsageRules.SemanticRelevanceThreshold == 0 {
		i.MemoryUsageRules.SemanticRelevanceThreshold = 0.65
	}
	if i.MemoryUsageRules.MaxContextMemories == 0 {
		i.MemoryUsageRules.MaxContextMemories = 15
	}
	if i.MemoryUsageRules.ReflectionInterval == 0 {
		i.MemoryUsageRules.ReflectionInterval = 5
	}
	if i.MemoryUsageRules.ReflectionHistory == 0 {
		i.MemoryUsageRules.ReflectionHistory = 20
	}
	if i.MemoryUsageRules.ClusterThreshold == 0 {
		i.MemoryUsageRules.ClusterThreshold = 0.70
	}
	if i.MemoryUsageRules.MinClusterSize == 0 {
		i.MemoryUsageRules.MinClusterSize = 2
	}
	if i.MemoryUsageRules.InterestTTLMinutes == 0 {
		i.MemoryUsageRules.InterestTTLMinutes = 10
	}
	if i.LLM.ChatTimeoutSeconds == 0 {
		i.LLM.ChatTimeoutSeconds = 60
	}
	if i.LLM.ReflectionTimeoutSeconds == 0 {
		i.LLM.ReflectionTimeoutSeconds = 120
	}
	if i.LLM.EmbedderTimeoutSeconds == 0 {
		i.LLM.EmbedderTimeoutSeconds = 30
	}
	if i.LLM.TranscriptionTimeoutSeconds == 0 {
		i.LLM.TranscriptionTimeoutSeconds = 120
	}
}
