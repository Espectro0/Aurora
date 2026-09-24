package capabilities

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/llm"
	"github.com/Espectro0/AuroraProject/internal/skills"
)

var _ skills.Skill = (*Skill)(nil)

type Lister interface {
	Definitions() []llm.ToolDefinition
}

type Skill struct {
	registry Lister
}

func New(registry Lister) *Skill {
	return &Skill{registry: registry}
}

func (s *Skill) Name() string {
	return "listar_capacidades"
}

func (s *Skill) Description() string {
	return "Lista las skills y capacidades disponibles de Aurora. Úsala cuando el usuario pregunte qué puedes hacer, qué skills o herramientas tienes."
}

func (s *Skill) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (s *Skill) Execute(ctx context.Context, argsJSON string) (skills.Result, error) {
	var natives []string
	mcp := map[string][]string{}

	for _, d := range s.registry.Definitions() {
		if d.Name == s.Name() {
			continue
		}
		desc := firstSentence(d.Description)

		if server, tool, ok := strings.Cut(d.Name, "__"); ok {
			mcp[server] = append(mcp[server], fmt.Sprintf("- %s: %s", tool, desc))
			continue
		}
		natives = append(natives, fmt.Sprintf("- %s: %s", d.Name, desc))
	}

	if len(natives) == 0 && len(mcp) == 0 {
		return skills.Result{Text: "No hay skills registradas."}, nil
	}

	var b strings.Builder
	if len(natives) > 0 {
		b.WriteString("Skills propias:\n")
		b.WriteString(strings.Join(natives, "\n"))
		b.WriteString("\n")
	}
	if len(mcp) > 0 {
		servers := make([]string, 0, len(mcp))
		for srv := range mcp {
			servers = append(servers, srv)
		}
		sort.Strings(servers)

		b.WriteString("\nHerramientas Externas (MCP):\n")
		for _, srv := range servers {
			fmt.Fprintf(&b, "- %s:\n%s\n", srv, strings.Join(mcp[srv], "\n"))
		}
	}

	return skills.Result{Text: b.String()}, nil
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ". "); i > 0 {
		return s[:i+1]
	}
	return s
}
