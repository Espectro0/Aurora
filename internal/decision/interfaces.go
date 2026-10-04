package decision

import "context"

type QuestionType string

const (
	TypeChoice QuestionType = "choice"
	TypeScore  QuestionType = "score"
	TypeNoul   QuestionType = "noul"
)

type Question struct {
	Type         QuestionType
	Instructions string
	Criteria     map[string]string
	Levels       []string
}

type Answer struct {
	Type          QuestionType
	Choice        string
	Score         float64
	Noul          float64
	Confidence    float64
	Probabilities map[string]float64
	Legend        map[string]string
}

type Provider interface {
	Judge(ctx context.Context, state string, questions map[string]Question) (map[string]Answer, error)
}
