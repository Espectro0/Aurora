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

type Confirmer interface {
	Confirm(ctx context.Context, req Request) (bool, error)
}

type confirmerKey struct{}

func WithConfirmer(ctx context.Context, c Confirmer) context.Context {
	return context.WithValue(ctx, confirmerKey{}, c)
}

func ConfirmerFrom(ctx context.Context) Confirmer {
	c, _ := ctx.Value(confirmerKey{}).(Confirmer)
	return c
}
