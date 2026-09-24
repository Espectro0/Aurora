package skills

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sync"

	"github.com/Espectro0/AuroraProject/internal/llm"
)

type Registry struct {
	mu     sync.RWMutex
	byName map[string]Skill
	order  []string
	group  map[string]string
	guard  Guard
}

type Guard interface {
	Run(ctx context.Context, s Skill, argsJSON string) (Result, error)
}

func (r *Registry) SetGuard(g Guard) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.guard = g
}

func NewRegistry() *Registry {
	return &Registry{
		byName: make(map[string]Skill),
		group:  make(map[string]string),
	}
}

func (r *Registry) Register(s Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.add(s, "")
}

func (r *Registry) ReplaceGroup(group string, skills []Skill) (skipped []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.removeGroup(group)
	for _, s := range skills {
		name := s.Name()
		if owner, exists := r.group[name]; exists && owner != group {
			log.Printf("[skills] %q from %q skiped: already exists (Group %q)", name, group, owner)
			skipped = append(skipped, name)
			continue
		}
		r.add(s, group)
	}
	return skipped
}

func (r *Registry) UnregisterGroup(group string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeGroup(group)
}

func (r *Registry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byName[name]
	return s, ok
}

func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byName)
}

func (r *Registry) Definitions() []llm.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]llm.ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		s := r.byName[name]
		defs = append(defs, llm.ToolDefinition{
			Name:        s.Name(),
			Description: s.Description(),
			Parameters:  s.Parameters(),
		})
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, argsJSON string) (Result, error) {
	r.mu.RLock()
	s, ok := r.byName[name]
	g := r.guard
	r.mu.RUnlock()

	if !ok {
		return Result{}, fmt.Errorf("skills: unknown skill %q", name)
	}
	if g != nil {
		return g.Run(ctx, s, argsJSON)
	}
	return s.Execute(ctx, argsJSON)
}

func (r *Registry) add(s Skill, group string) {
	name := s.Name()
	if _, exists := r.byName[name]; !exists {
		r.order = append(r.order, name)
	}
	r.byName[name] = s
	r.group[name] = group
}

func (r *Registry) removeGroup(group string) {
	if group == "" {
		return
	}
	r.order = slices.DeleteFunc(r.order, func(name string) bool {
		if r.group[name] != group {
			return false
		}
		delete(r.byName, name)
		delete(r.group, name)
		return true
	})
}
