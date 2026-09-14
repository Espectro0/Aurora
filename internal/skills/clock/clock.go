package clock

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Espectro0/AuroraProject/internal/skills"
)

var _ skills.Skill = (*Skill)(nil)

type Skill struct{}

func New() *Skill {
	return &Skill{}
}

func (s *Skill) Name() string {
	return "hora_actual"
}

func (s *Skill) Description() string {
	return "Devuelve la fecha y hora actual. Úsala solo si el usuario pregunta explícitamente qué hora o fecha es en este momento."
}

func (s *Skill) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (s *Skill) Execute(ctx context.Context, argsJSON string) (skills.Result, error) {
	now := time.Now()
	out, err := json.Marshal(map[string]string{
		"datetime": now.Format(time.RFC3339),
		"weekday":  now.Weekday().String(),
	})
	if err != nil {
		return skills.Result{}, err
	}
	return skills.Result{Text: string(out)}, nil
}
