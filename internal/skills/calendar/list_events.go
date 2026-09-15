package calendar

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Espectro0/AuroraProject/internal/skills"
)

var _ skills.Skill = (*ListEventsSkill)(nil)

type ListEventsSkill struct {
	client *Client
}

func NewListEvents(client *Client) *ListEventsSkill {
	return &ListEventsSkill{client: client}
}

func (s *ListEventsSkill) Name() string {
	return "calendar_list_events"
}

func (s *ListEventsSkill) Description() string {
	return "Lista eventos de un calendario de Apple Calendar, con filtros opcionales de rango de fechas y búsqueda. " +
		"Requiere el identificador del calendario (obtenido con calendar_list_calendars). " +
		"Úsala cuando el usuario pregunte qué eventos tiene, o pida buscar un evento existente."
}

func (s *ListEventsSkill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"calendarId": map[string]any{
				"type":        "string",
				"description": "Identificador del calendario a consultar (ver calendar_list_calendars).",
			},
			"startDateTime": map[string]any{
				"type":        "string",
				"description": "Fecha/hora mínima (ISO 8601) para filtrar eventos. Opcional.",
			},
			"endDateTime": map[string]any{
				"type":        "string",
				"description": "Fecha/hora máxima (ISO 8601) para filtrar eventos. Opcional.",
			},
			"timeZone": map[string]any{
				"type":        "string",
				"description": "Zona horaria IANA a usar para interpretar las fechas (ej. 'America/Bogota'). Opcional.",
			},
			"search": map[string]any{
				"type":        "string",
				"description": "Texto libre para buscar en título/descripción de los eventos. Opcional.",
			},
			"expandRecurrences": map[string]any{
				"type":        "boolean",
				"description": "Si es true, expande eventos recurrentes en instancias individuales. Opcional.",
			},
			"pageSize": map[string]any{
				"type":        "integer",
				"description": "Número máximo de eventos a devolver. Opcional.",
			},
			"pageToken": map[string]any{
				"type":        "string",
				"description": "Token de paginación devuelto por una llamada anterior. Opcional.",
			},
		},
		"required": []string{"calendarId"},
	}
}

type listEventsArgs struct {
	CalendarID        string `json:"calendarId"`
	StartDateTime     string `json:"startDateTime"`
	EndDateTime       string `json:"endDateTime"`
	TimeZone          string `json:"timeZone"`
	Search            string `json:"search"`
	ExpandRecurrences bool   `json:"expandRecurrences"`
	PageSize          int    `json:"pageSize"`
	PageToken         string `json:"pageToken"`
}

func (s *ListEventsSkill) Execute(ctx context.Context, argsJSON string) (skills.Result, error) {
	var args listEventsArgs
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return skills.Result{}, fmt.Errorf("calendar_list_events: parseando argumentos: %w", err)
		}
	}
	if args.CalendarID == "" {
		return skills.Result{}, fmt.Errorf("calendar_list_events: calendarId es requerido")
	}

	result, err := s.client.ListEvents(ctx, args.CalendarID, ListEventsOptions{
		PageSize:          args.PageSize,
		PageToken:         args.PageToken,
		StartDateTime:     args.StartDateTime,
		EndDateTime:       args.EndDateTime,
		TimeZone:          args.TimeZone,
		ExpandRecurrences: args.ExpandRecurrences,
		Search:            args.Search,
	})
	if err != nil {
		return skills.Result{}, err
	}

	out, err := json.Marshal(map[string]any{
		"total":         len(result.Events),
		"events":        result.Events,
		"nextPageToken": result.NextPageToken,
	})
	if err != nil {
		return skills.Result{}, err
	}

	return skills.Result{Text: string(out)}, nil
}
