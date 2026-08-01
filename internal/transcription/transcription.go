package transcription

import "context"

type Provider interface {
	Transcribe(ctx context.Context, audio []byte, filename string) (string, error)
}
