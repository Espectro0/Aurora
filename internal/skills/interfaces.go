package skills

import "context"

type Skill interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, argsJSON string) (string, error)
}
