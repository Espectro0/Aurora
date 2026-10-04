package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/skills"
	"github.com/Espectro0/AuroraProject/internal/voice"
)

const voiceUserID = "panel:voice"

const maxVoiceClipBytes = 10 << 20

type Replier interface {
	Reply(ctx context.Context, userID string, message string) (string, []skills.Attachment, error)
}

type VoiceHandler struct {
	agent Replier
	voice *voice.Client
	token string
}

func NewVoiceHandler(agent Replier, v *voice.Client, token string) *VoiceHandler {
	return &VoiceHandler{agent: agent, voice: v, token: token}
}

type voiceResponse struct {
	Transcript string `json:"transcript"`
	Reply      string `json:"reply"`
	Audio      string `json:"audio,omitempty"`
	SpeechErr  string `json:"speech_error,omitempty"`
	Timings    struct {
		TranscribeMs int64 `json:"transcribe_ms"`
		ReplyMs      int64 `json:"reply_ms"`
		SpeechMs     int64 `json:"speech_ms"`
	} `json:"timings"`
}

func (h *VoiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(got), []byte(h.token)) != 1 {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	clip, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxVoiceClipBytes))
	if err != nil {
		http.Error(w, "clip too large (max 10 MB)", http.StatusRequestEntityTooLarge)
		return
	}
	format := audioFormat(clip)
	if format == "" {
		http.Error(w, "expected WebM or Ogg audio", http.StatusUnsupportedMediaType)
		return
	}

	var out voiceResponse
	ctx := r.Context()

	start := time.Now()
	out.Transcript, err = h.voice.Transcribe(ctx, clip, format)
	out.Timings.TranscribeMs = time.Since(start).Milliseconds()
	if err != nil {
		log.Printf("[voice] transcribe: %v", err)
		http.Error(w, "transcription failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	if out.Transcript == "" {
		http.Error(w, "no speech detected", http.StatusUnprocessableEntity)
		return
	}

	start = time.Now()
	out.Reply, _, err = h.agent.Reply(ctx, voiceUserID, out.Transcript)
	out.Timings.ReplyMs = time.Since(start).Milliseconds()
	if err != nil {
		log.Printf("[voice] reply: %v", err)
		http.Error(w, "aurora failed to reply: "+err.Error(), http.StatusBadGateway)
		return
	}

	if spoken := speakable(out.Reply); spoken != "" {
		start = time.Now()
		audio, err := h.voice.Speak(ctx, spoken)
		out.Timings.SpeechMs = time.Since(start).Milliseconds()
		if err != nil {
			log.Printf("[voice] speak: %v", err)
			out.SpeechErr = err.Error()
		} else {
			out.Audio = base64.StdEncoding.EncodeToString(audio)
		}
	}

	log.Printf("[voice] %q → %q (stt %dms, reply %dms, tts %dms)", out.Transcript, out.Reply,
		out.Timings.TranscribeMs, out.Timings.ReplyMs, out.Timings.SpeechMs)
	writeJSON(w, out)
}

func audioFormat(clip []byte) string {
	switch {
	case bytes.HasPrefix(clip, []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "webm"
	case bytes.HasPrefix(clip, []byte("OggS")):
		return "ogg"
	}
	return ""
}

var (
	mdLink   = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	mdMarks  = regexp.MustCompile("[*_`#>~]+")
	bareURLs = regexp.MustCompile(`https?://\S+`)
)

func speakable(s string) string {
	s = mdLink.ReplaceAllString(s, "$1")
	s = bareURLs.ReplaceAllString(s, "")
	s = mdMarks.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
