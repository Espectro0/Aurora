package calendar

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Espectro0/AuroraProject/internal/skills"
)

var _ skills.Skill = (*GetEventSkill)(nil)

type GetEventSkill struct {
	client *Client
}

func NewGetEvent(client *Client) *GetEventSkill {
	return &GetEventSkill{client: client}
}

func (s *GetEventSkill) Name() string {
	return "calendar_get_event"
}

func (s *GetEventSkill) Description() string {
	return "Obtiene el detalle completo de un evento específico de Apple Calendar. " +
		"Requiere el identificador del calendario y del evento (obtenidos con calendar_list_calendars y calendar_list_events). " +
		"Úsala cuando el usuario pida detalles de un evento puntual."
}

func (s *GetEventSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"calendarId": map[string]any{
				"type":        "string",
				"description": "Identificador del calendario que contiene el evento.",
			},
			"eventId": map[string]any{
				"type":        "string",
				"description": "Identificador del evento a consultar.",
			},
		},
		"required": []string{"calendarId", "eventId"},
	}
}

type getEventArgs struct {
	CalendarID string `json:"calendarId"`
	EventID    string `json:"eventId"`
}

func (s *GetEventSkill) Execute(ctx context.Context, argsJSON string) (skills.Result, error) {
	var args getEventArgs
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return skills.Result{}, fmt.Errorf("calendar_get_event: parseando argumentos: %w", err)
		}
	}
	if args.CalendarID == "" || args.EventID == "" {
		return skills.Result{}, fmt.Errorf("calendar_get_event: calendarId y eventId son requeridos")
	}

	event, err := s.client.GetEvent(ctx, args.CalendarID, args.EventID)
	if err != nil {
		return skills.Result{}, err
	}

	out, err := json.Marshal(event)
	if err != nil {
		return skills.Result{}, err
	}

	return skills.Result{Text: string(out)}, nil
}
