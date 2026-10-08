package conversation

import "context"

type Speaker struct {
	ID      string
	Name    string
	Owner   bool
	Bot     bool
	Mention string
}

type speakerKey struct{}

func WithSpeaker(ctx context.Context, s Speaker) context.Context {
	return context.WithValue(ctx, speakerKey{}, s)
}

func SpeakerFrom(ctx context.Context) (Speaker, bool) {
	s, ok := ctx.Value(speakerKey{}).(Speaker)
	return s, ok
}
