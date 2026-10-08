package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Espectro0/AuroraProject/internal/conversation"
	"github.com/Espectro0/AuroraProject/internal/guard"
	"github.com/Espectro0/AuroraProject/internal/skills"
	"github.com/Espectro0/AuroraProject/internal/voice"
)

const voiceUserID = "panel:voice"

const maxVoiceClipBytes = 10 << 20

type Replier interface {
	Reply(ctx context.Context, userID string, message string) (string, []skills.Attachment, error)
}

type VoiceHandler struct {
	agent   Replier
	voice   *voice.Client
	token   string
	pending *guard.Pending
}

func NewVoiceHandler(agent Replier, v *voice.Client, token string) *VoiceHandler {
	return &VoiceHandler{agent: agent, voice: v, token: token, pending: guard.NewPending()}
}

type voiceEvent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Ms   int64  `json:"ms,omitempty"`

	Audio string `json:"audio,omitempty"`
	Error string `json:"error,omitempty"`

	ID        string `json:"id,omitempty"`
	Skill     string `json:"skill,omitempty"`
	Server    string `json:"server,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
	Approved  *bool  `json:"approved,omitempty"`
}

type eventStream struct {
	mu sync.Mutex
	w  http.ResponseWriter
	f  http.Flusher
}

func (s *eventStream) send(e voiceEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	line, _ := json.Marshal(e)
	s.w.Write(append(line, '\n'))
	s.f.Flush()
}

func (h *VoiceHandler) authorized(r *http.Request) bool {
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return subtle.ConstantTimeCompare([]byte(got), []byte(h.token)) == 1
}

func (h *VoiceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
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

	ctx := r.Context()

	start := time.Now()
	transcript, err := h.voice.Transcribe(ctx, clip, format)
	if err != nil {
		log.Printf("[voice] transcribe: %v", err)
		http.Error(w, "transcription failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	if transcript == "" {
		http.Error(w, "no speech detected", http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	out := &eventStream{w: w, f: flusher}
	out.send(voiceEvent{Type: "transcript", Text: transcript, Ms: time.Since(start).Milliseconds()})

	ctx = conversation.WithSpeaker(ctx, conversation.Speaker{ID: voiceUserID, Owner: true})
	ctx = guard.WithConfirmer(ctx, &panelConfirmer{h: h, out: out})

	start = time.Now()
	reply, _, err := h.agent.Reply(ctx, voiceUserID, transcript)
	if err != nil {
		log.Printf("[voice] reply: %v", err)
		out.send(voiceEvent{Type: "error", Error: "aurora failed to reply: " + err.Error()})
		return
	}
	out.send(voiceEvent{Type: "reply", Text: reply, Ms: time.Since(start).Milliseconds()})
	log.Printf("[voice] %q → %q", transcript, reply)

	spoken := speakable(reply)
	if spoken == "" {
		return
	}
	start = time.Now()
	audio, err := h.voice.Speak(ctx, spoken)
	if err != nil {
		log.Printf("[voice] speak: %v", err)
		out.send(voiceEvent{Type: "speech_error", Error: err.Error()})
		return
	}
	out.send(voiceEvent{Type: "audio", Audio: base64.StdEncoding.EncodeToString(audio), Ms: time.Since(start).Milliseconds()})
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
