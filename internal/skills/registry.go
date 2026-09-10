package skills

import (
	"context"
	"fmt"

	"github.com/Espectro0/AuroraProject/internal/llm"
)

type Registry struct {
	byName map[string]Skill
	order  []string
}

func NewRegistry() *Registry {
	return &Registry{
		byName: make(map[string]Skill),
	}
}

func (r *Registry) Register(s Skill) {
	name := s.Name()
	if _, exists := r.byName[name]; !exists {
		r.order = append(r.order, name)
	}
	r.byName[name] = s
}

func (r *Registry) Get(name string) (Skill, bool) {
	s, ok := r.byName[name]
	return s, ok
}

func (r *Registry) Len() int {
	return len(r.byName)
}

func (r *Registry) Definitions() []llm.ToolDefinition {
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

func (r *Registry) Execute(ctx context.Context, name string, argsJSON string) (string, error) {
	s, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("skills: unknown skill %q", name)
	}
	return s.Execute(ctx, argsJSON)
}
