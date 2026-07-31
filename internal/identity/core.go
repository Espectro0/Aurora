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
	return json.Unmarshal(raw, &c.identity)
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
		},
	}
}
