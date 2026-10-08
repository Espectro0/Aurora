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

func (c *Core) Update(fn func(*IdentityCore)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	prev := c.identity
	next := c.identity
	next.Values = append([]string(nil), prev.Values...)
	next.ConversationalPrinciples = append([]string(nil), prev.ConversationalPrinciples...)
	next.FoundationalMemories = append([]string(nil), prev.FoundationalMemories...)
	fn(&next)

	c.identity = next
	if err := c.save(); err != nil {
		c.identity = prev
		return err
	}
	return nil
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
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
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
			ImportanceWeight:           0.1,
			ReflectionGateThreshold:    0.4,
			WorthKeepingThreshold:      0.5,
			SameEntityThreshold:        0.7,
			NodeReplaceThreshold:       0.8,
			IdentityChangeThreshold:    0.85,
			CuriosityThreshold:         0.6,
			CuriosityCooldownHours:     24,
		},
		LLM: LLMConfig{
			ChatTimeoutSeconds:       60,
			ReflectionTimeoutSeconds: 120,
			EmbedderTimeoutSeconds:   30,
		},
		Guard: GuardConfig{
			ConfirmTimeoutSeconds: 120,
		},
		Sleep: SleepConfig{
			Hour:                  3,
			MergeThreshold:        0.92,
			DecayHalfLifeDays:     30,
			RecallBoost:           0.1,
			ForgetThreshold:       0.15,
			ForgetMinAgeDays:      14,
			MaxInterests:          5,
			TimeoutMinutes:        20,
			MaxExplorations:       3,
			ExploreCooldownDays:   7,
			MaxSyntheses:          5,
			SynthesisThreshold:    0.6,
			SynthesisCooldownDays: 7,
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
	if i.MemoryUsageRules.ReflectionGateThreshold == 0 {
		i.MemoryUsageRules.ReflectionGateThreshold = 0.4
	}
	if i.MemoryUsageRules.WorthKeepingThreshold == 0 {
		i.MemoryUsageRules.WorthKeepingThreshold = 0.5
	}
	if i.MemoryUsageRules.SameEntityThreshold == 0 {
		i.MemoryUsageRules.SameEntityThreshold = 0.7
	}
	if i.MemoryUsageRules.NodeReplaceThreshold == 0 {
		i.MemoryUsageRules.NodeReplaceThreshold = 0.8
	}
	if i.MemoryUsageRules.IdentityChangeThreshold == 0 {
		i.MemoryUsageRules.IdentityChangeThreshold = 0.85
	}
	if i.MemoryUsageRules.CuriosityThreshold == 0 {
		i.MemoryUsageRules.CuriosityThreshold = 0.6
	}
	if i.MemoryUsageRules.CuriosityCooldownHours == 0 {
		i.MemoryUsageRules.CuriosityCooldownHours = 24
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
	if i.Guard.ConfirmTimeoutSeconds == 0 {
		i.Guard.ConfirmTimeoutSeconds = 120
	}
	if i.Sleep.Hour < 0 || i.Sleep.Hour > 23 {
		i.Sleep.Hour = 3
	}
	if i.Sleep.MergeThreshold == 0 {
		i.Sleep.MergeThreshold = 0.92
	}
	if i.Sleep.DecayHalfLifeDays == 0 {
		i.Sleep.DecayHalfLifeDays = 30
	}
	if i.Sleep.RecallBoost == 0 {
		i.Sleep.RecallBoost = 0.1
	}
	if i.Sleep.ForgetThreshold == 0 {
		i.Sleep.ForgetThreshold = 0.15
	}
	if i.Sleep.ForgetMinAgeDays == 0 {
		i.Sleep.ForgetMinAgeDays = 14
	}
	if i.Sleep.MaxInterests == 0 {
		i.Sleep.MaxInterests = 5
	}
	if i.Sleep.TimeoutMinutes == 0 {
		i.Sleep.TimeoutMinutes = 20
	}
	if i.Sleep.MaxExplorations == 0 {
		i.Sleep.MaxExplorations = 3
	}
	if i.Sleep.ExploreCooldownDays == 0 {
		i.Sleep.ExploreCooldownDays = 7
	}
	if i.Sleep.MaxSyntheses == 0 {
		i.Sleep.MaxSyntheses = 5
	}
	if i.Sleep.SynthesisThreshold == 0 {
		i.Sleep.SynthesisThreshold = 0.6
	}
	if i.Sleep.SynthesisCooldownDays == 0 {
		i.Sleep.SynthesisCooldownDays = 7
	}
}
