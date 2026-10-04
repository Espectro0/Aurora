package voice

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultSampleRate = 24000

type Config struct {
	BaseURL  string
	APIKey   string
	STTModel string
	TTSModel string
	Voice    string
	Language string
}

type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config, timeout time.Duration) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: timeout}}
}

func (c *Client) Transcribe(ctx context.Context, audio []byte, format string) (string, error) {
	body := map[string]any{
		"model": c.cfg.STTModel,
		"input_audio": map[string]string{
			"data":   base64.StdEncoding.EncodeToString(audio),
			"format": format,
		},
	}
	if c.cfg.Language != "" {
		body["language"] = c.cfg.Language
	}

	resp, err := c.post(ctx, "/audio/transcriptions", body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("voice: decode transcription: %w", err)
	}
	return strings.TrimSpace(result.Text), nil
}

func (c *Client) Speak(ctx context.Context, text string) ([]byte, error) {
	resp, err := c.post(ctx, "/audio/speech", map[string]any{
		"model":           c.cfg.TTSModel,
		"input":           text,
		"voice":           c.cfg.Voice,
		"response_format": "pcm",
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	pcm, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("voice: read speech: %w", err)
	}

	rate, channels := defaultSampleRate, 1
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type")); err == nil {
		if v, err := strconv.Atoi(params["rate"]); err == nil && v > 0 {
			rate = v
		}
		if v, err := strconv.Atoi(params["channels"]); err == nil && v > 0 {
			channels = v
		}
	}
	return wav(pcm, rate, channels), nil
}

func (c *Client) post(ctx context.Context, path string, body any) (*http.Response, error) {
	jsonBody, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("voice: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voice: %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("voice: %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return resp, nil
}

func wav(pcm []byte, rate, channels int) []byte {
	const bits = 16
	blockAlign := channels * bits / 8

	var b bytes.Buffer
	b.Grow(44 + len(pcm))
	b.WriteString("RIFF")
	binary.Write(&b, binary.LittleEndian, uint32(36+len(pcm)))
	b.WriteString("WAVEfmt ")
	binary.Write(&b, binary.LittleEndian, uint32(16))
	binary.Write(&b, binary.LittleEndian, uint16(1))
	binary.Write(&b, binary.LittleEndian, uint16(channels))
	binary.Write(&b, binary.LittleEndian, uint32(rate))
	binary.Write(&b, binary.LittleEndian, uint32(rate*blockAlign))
	binary.Write(&b, binary.LittleEndian, uint16(blockAlign))
	binary.Write(&b, binary.LittleEndian, uint16(bits))
	b.WriteString("data")
	binary.Write(&b, binary.LittleEndian, uint32(len(pcm)))
	b.Write(pcm)
	return b.Bytes()
}
