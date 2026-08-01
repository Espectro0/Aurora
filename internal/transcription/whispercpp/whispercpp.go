package whispercpp

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/transcription"
)

var _ transcription.Provider = (*Client)(nil)

type Client struct {
	binPath   string
	model     string
	language  string
	ffmpegBin string
	sem       chan struct{}
}

func New(binPath, model, language, ffmpegBin string) *Client {
	return &Client{
		binPath:   binPath,
		model:     model,
		language:  language,
		ffmpegBin: ffmpegBin,
		sem:       make(chan struct{}, 1),
	}
}

func (c *Client) Transcribe(ctx context.Context, audio []byte, filename string) (string, error) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return "", fmt.Errorf("whispercpp: %w", ctx.Err())
	}

	dir, err := os.MkdirTemp("", "aurora-whisper-*")
	if err != nil {
		return "", fmt.Errorf("whispercpp: %w", err)
	}
	if os.Getenv("AURORA_KEEP_WAV") == "" {
		defer os.RemoveAll(dir)
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".wav"
	}
	inPath := filepath.Join(dir, "input"+ext)
	if err := os.WriteFile(inPath, audio, 0o600); err != nil {
		return "", fmt.Errorf("whispercpp: write audio: %w", err)
	}
	if os.Getenv("AURORA_KEEP_WAV") != "" {
		log.Printf("[whispercpp] keeping wav: %s", inPath)
	}

	var stderr bytes.Buffer
	if c.ffmpegBin != "" && ext != ".wav" {
		wavPath := filepath.Join(dir, "input.wav")
		ffmpeg := exec.CommandContext(ctx, c.ffmpegBin, "-y", "-loglevel", "error", "-i", inPath, "-ar", "16000", "-ac", "1", "-f", "wav", wavPath)
		ffmpeg.Stderr = &stderr
		if err := ffmpeg.Run(); err != nil {
			if ctx.Err() != nil {
				return "", fmt.Errorf("whispercpp: %w", ctx.Err())
			}
			return "", fmt.Errorf("whispercpp: convert to wav: %w: %s", err, tail(stderr.String()))
		}
		inPath = wavPath
	}

	outBase := filepath.Join(dir, "output")
	cmd := exec.CommandContext(ctx, c.binPath,
		"-m", c.model,
		"-f", inPath,
		"-l", c.language,
		"-t", strconv.Itoa(runtime.NumCPU()),
		"-otxt",
		"-of", outBase,
		"-np",
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("whispercpp: %w", ctx.Err())
		}
		return "", fmt.Errorf("whispercpp: run whisper-cli: %w: %s", err, tail(stderr.String()))
	}

	text, err := os.ReadFile(outBase + ".txt")
	if err != nil {
		return "", fmt.Errorf("whispercpp: read output: %w (stderr: %s)", err, tail(stderr.String()))
	}
	return strings.TrimSpace(string(text)), nil
}

func tail(s string) string {
	const max = 500
	if len(s) <= max {
		return s
	}
	return "..." + s[len(s)-max:]
}
