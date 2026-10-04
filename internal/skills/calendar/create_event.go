package calendar

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Espectro0/AuroraProject/internal/skills"
)

var _ skills.Skill = (*CreateEventSkill)(nil)

type CreateEventSkill struct {
	client *Client
}

func NewCreateEvent(client *Client) *CreateEventSkill {
	return &CreateEventSkill{client: client}
}

func (s *CreateEventSkill) Name() string {
	return "calendar_create_event"
}

func (s *CreateEventSkill) Description() string {
	return "Crea un nuevo evento en un calendario de Apple Calendar. " +
		"Requiere el identificador del calendario (obtenido con calendar_list_calendars), un título y una fecha/hora de inicio. " +
		"Úsala cuando el usuario pida agendar, programar o crear un evento."
}

func (s *CreateEventSkill) Parameters() map[string]any {
	dateTimeSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"dateTime": map[string]any{
				"type":        "string",
				"description": "Fecha y hora en formato ISO 8601 (ej. '2026-09-15T10:00:00').",
			},
			"timeZone": map[string]any{
				"type":        "string",
				"description": "Zona horaria IANA (ej. 'America/Bogota'). Opcional.",
			},
		},
		"required": []string{"dateTime"},
	}

	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"calendarId": map[string]any{
				"type":        "string",
				"description": "Identificador del calendario donde crear el evento.",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Título del evento.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Descripción o notas del evento. Opcional.",
			},
			"start": dateTimeSchema,
			"end": map[string]any{
				"type":        "object",
				"description": "Fecha/hora de fin del evento. Opcional.",
				"properties":  dateTimeSchema["properties"],
			},
			"isAllDay": map[string]any{
				"type":        "boolean",
				"description": "Si es true, el evento dura todo el día. Opcional.",
			},
			"location": map[string]any{
				"type":        "string",
				"description": "Lugar del evento. Opcional.",
			},
			"attendees": map[string]any{
				"type":        "array",
				"description": "Lista de invitados al evento. Opcional.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":  map[string]any{"type": "string"},
						"email": map[string]any{"type": "string"},
					},
				},
			},
		},
		"required": []string{"calendarId", "title", "start"},
	}
}

type createEventArgs struct {
	CalendarID  string         `json:"calendarId"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Start       EventDateTime  `json:"start"`
	End         *EventDateTime `json:"end,omitempty"`
	IsAllDay    bool           `json:"isAllDay"`
	Location    string         `json:"location"`
	Attendees   []Attendee     `json:"attendees"`
}

func (s *CreateEventSkill) Execute(ctx context.Context, argsJSON string) (skills.Result, error) {
	var args createEventArgs
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return skills.Result{}, fmt.Errorf("calendar_create_event: parseando argumentos: %w", err)
		}
	}
	if args.CalendarID == "" || args.Title == "" || args.Start.DateTime == "" {
		return skills.Result{}, fmt.Errorf("calendar_create_event: calendarId, title y start.dateTime son requeridos")
	}

	event, err := s.client.CreateEvent(ctx, args.CalendarID, CreateEventRequest{
		Title:       args.Title,
		Description: args.Description,
		Start:       args.Start,
		End:         args.End,
		IsAllDay:    args.IsAllDay,
		Location:    args.Location,
		Attendees:   args.Attendees,
	})
	if err != nil {
		return skills.Result{}, err
	}

	out, err := json.Marshal(event)
	if err != nil {
		return skills.Result{}, err
	}

	return skills.Result{Text: string(out)}, nil
}
