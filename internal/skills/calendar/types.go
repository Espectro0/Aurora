package calendar

import "net/http"

type Calendar struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ReadOnly bool   `json:"readOnly"`
	HexColor string `json:"hexColor"`
	IsShared bool   `json:"isShared"`
	TimeZone string `json:"timeZone"`
}

type listCalendarsResponse struct {
	Data          []Calendar `json:"data"`
	NextSyncToken string     `json:"nextSyncToken"`
}

type EventDateTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone,omitempty"`
}

type Attendee struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	Email          string `json:"email,omitempty"`
	ResponseStatus string `json:"responseStatus,omitempty"`
}

type Event struct {
	ID                         string         `json:"id,omitempty"`
	Title                      string         `json:"title"`
	Description                string         `json:"description,omitempty"`
	ETag                       string         `json:"etag,omitempty"`
	CreatedAt                  string         `json:"createdAt,omitempty"`
	UpdatedAt                  string         `json:"updatedAt,omitempty"`
	Start                      EventDateTime  `json:"start"`
	End                        *EventDateTime `json:"end,omitempty"`
	IsAllDay                   bool           `json:"isAllDay,omitempty"`
	IsRecurring                bool           `json:"isRecurring,omitempty"`
	IsException                bool           `json:"isException,omitempty"`
	IsCancelled                bool           `json:"isCancelled,omitempty"`
	Recurrence                 []string       `json:"recurrence,omitempty"`
	MyResponseStatus           string         `json:"myResponseStatus,omitempty"`
	Attendees                  []Attendee     `json:"attendees,omitempty"`
	Visibility                 string         `json:"visibility,omitempty"`
	Location                   string         `json:"location,omitempty"`
	ConferenceData             map[string]any `json:"conferenceData,omitempty"`
	WebLink                    string         `json:"webLink,omitempty"`
	ICalUID                    string         `json:"iCalUid,omitempty"`
	Transparency               string         `json:"transparency,omitempty"`
	EventType                  string         `json:"eventType,omitempty"`
	Organizer                  map[string]any `json:"organizer,omitempty"`
	Creator                    map[string]any `json:"creator,omitempty"`
	Reminders                  map[string]any `json:"reminders,omitempty"`
	GenerateMeetingURLProvider string         `json:"generateMeetingUrlProvider,omitempty"`
	PublicExtendedProperties   map[string]any `json:"publicExtendedProperties,omitempty"`
	PrivateExtendedProperties  map[string]any `json:"privateExtendedProperties,omitempty"`
}

type CreateEventRequest struct {
	Title                      string         `json:"title"`
	Description                string         `json:"description,omitempty"`
	Start                      EventDateTime  `json:"start"`
	End                        *EventDateTime `json:"end,omitempty"`
	IsAllDay                   bool           `json:"isAllDay,omitempty"`
	Location                   string         `json:"location,omitempty"`
	Visibility                 string         `json:"visibility,omitempty"`
	EventType                  string         `json:"eventType,omitempty"`
	Recurrence                 []string       `json:"recurrence,omitempty"`
	Attendees                  []Attendee     `json:"attendees,omitempty"`
	Reminders                  map[string]any `json:"reminders,omitempty"`
	ConferenceData             map[string]any `json:"conferenceData,omitempty"`
	PublicExtendedProperties   map[string]any `json:"publicExtendedProperties,omitempty"`
	PrivateExtendedProperties  map[string]any `json:"privateExtendedProperties,omitempty"`
}

type ListEventsOptions struct {
	PageSize          int
	PageToken         string
	SyncToken         string
	StartDateTime     string
	EndDateTime       string
	TimeZone          string
	MetadataFilters   string
	ExpandRecurrences bool
	Search            string
	OrderBy           string
}

type ListEventsResult struct {
	Events        []Event `json:"data"`
	NextSyncToken string  `json:"nextSyncToken"`
	NextPageToken string  `json:"nextPageToken"`
}

type Client struct {
	http             *http.Client
	baseURL          string
	apiKey           string
	endUserAccountID string
}