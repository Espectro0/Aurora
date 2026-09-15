package calendar

import (
	"context"
	"encoding/json"

	"github.com/Espectro0/AuroraProject/internal/skills"
)

var _ skills.Skill = (*ListCalendarsSkill)(nil)

type ListCalendarsSkill struct {
	client *Client
}

func NewListCalendars(client *Client) *ListCalendarsSkill {
	return &ListCalendarsSkill{client: client}
}

func (s *ListCalendarsSkill) Name() string {
	return "calendar_list_calendars"
}

func (s *ListCalendarsSkill) Description() string {
	return "Lista los calendarios disponibles en Apple Calendar. " +
		"Úsala cuando el usuario pregunte qué calendarios tiene disponibles, o antes de buscar o crear eventos " +
		"si aún no sabes el identificador del calendario correcto."
}

func (s *ListCalendarsSkill) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (s *ListCalendarsSkill) Execute(ctx context.Context, argsJSON string) (skills.Result, error) {
	calendars, err := s.client.ListCalendars(ctx)
	if err != nil {
		return skills.Result{}, err
	}

	out, err := json.Marshal(map[string]any{
		"total":     len(calendars),
		"calendars": calendars,
	})
	if err != nil {
		return skills.Result{}, err
	}

	return skills.Result{Text: string(out)}, nil
}
