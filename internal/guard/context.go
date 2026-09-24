package guard

import "context"

type userMessageKey struct{}

func WithUserMessage(ctx context.Context, msg string) context.Context {
	return context.WithValue(ctx, userMessageKey{}, msg)
}

func UserMessage(ctx context.Context) string {
	s, _ := ctx.Value(userMessageKey{}).(string)
	return s
}
