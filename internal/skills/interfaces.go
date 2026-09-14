package skills

import "context"

type Attachment struct {
	Path     string
	Filename string
}

type Result struct {
	Text        string
	Attachments []Attachment
}

type Skill interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, argsJSON string) (Result, error)
}
