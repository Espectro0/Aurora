package sleep

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/Espectro0/AuroraProject/internal/skills"
	"github.com/Espectro0/AuroraProject/internal/sleep"
)

var _ skills.Skill = (*Skill)(nil)

const waitForReport = 45 * time.Second

type Skill struct {
	sleeper *sleep.Sleeper
}

func New(s *sleep.Sleeper) *Skill {
	return &Skill{sleeper: s}
}

func (s *Skill) Name() string {
	return "dormir"
}

func (s *Skill) Description() string {
	return "Inicia tu ciclo de sueño: fusionas recuerdos duplicados, se desvanece lo que nunca recuerdas, olvidas lo menos importante y recalculas tus intereses emergentes. Ya ocurre sola cada noche; úsala solo si el usuario pide explícitamente que duermas, descanses u ordenes tu memoria ahora."
}

func (s *Skill) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

type result struct {
	Report *sleep.Report `json:"report,omitempty"`
	Status string        `json:"status"`
}

func (s *Skill) Execute(ctx context.Context, argsJSON string) (skills.Result, error) {
	type outcome struct {
		report sleep.Report
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		r, err := s.sleeper.Run(context.WithoutCancel(ctx))
		done <- outcome{r, err}
	}()

	var res result
	select {
	case o := <-done:
		if errors.Is(o.err, sleep.ErrAlreadySleeping) {
			res.Status = "ya estabas durmiendo; el ciclo en curso terminará solo"
		} else if o.err != nil {
			return skills.Result{}, o.err
		} else {
			res.Status = "dormiste y despertaste; el resumen también quedó en tu diario"
			res.Report = &o.report
		}
	case <-time.After(waitForReport):
		res.Status = "sigues durmiendo; el resumen quedará en tu diario al despertar"
	case <-ctx.Done():
		return skills.Result{}, ctx.Err()
	}

	out, err := json.Marshal(res)
	if err != nil {
		return skills.Result{}, err
	}
	return skills.Result{Text: string(out)}, nil
}
