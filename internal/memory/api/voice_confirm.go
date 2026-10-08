package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"unicode"

	"github.com/Espectro0/AuroraProject/internal/guard"
)

type panelConfirmer struct {
	h   *VoiceHandler
	out *eventStream
}

func (c *panelConfirmer) Confirm(ctx context.Context, req guard.Request) (bool, error) {
	id, answer := c.h.pending.Add(voiceUserID)
	defer c.h.pending.Drop(id)

	ev := voiceEvent{
		Type:      "confirm",
		ID:        id,
		Text:      req.Text(),
		Skill:     req.Call.Skill,
		Server:    req.Call.Server,
		Sensitive: req.Sensitive,
	}
	if audio, err := c.h.voice.Speak(ctx, fmt.Sprintf("Necesito tu confirmación para ejecutar %s. ¿Sí o no?", spokenName(req.Call.Skill))); err != nil {
		log.Printf("[voice] speak confirmation: %v", err)
	} else {
		ev.Audio = base64.StdEncoding.EncodeToString(audio)
	}
	c.out.send(ev)

	ok, err := guard.Await(ctx, answer)
	result := voiceEvent{Type: "confirm_result", ID: id}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		result.Text = "expired"
	case err != nil:
		result.Text = "cancelled"
	default:
		result.Approved = &ok
	}
	c.out.send(result)
	return ok, err
}

type confirmResponse struct {
	Transcript string `json:"transcript,omitempty"`
	Decision   string `json:"decision"`
	Audio      string `json:"audio,omitempty"`
}

func (h *VoiceHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	var out confirmResponse

	switch r.URL.Query().Get("answer") {
	case "yes":
		out.Decision = "yes"
	case "no":
		out.Decision = "no"
	case "":
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
		out.Transcript, err = h.voice.Transcribe(r.Context(), clip, format)
		if err != nil {
			log.Printf("[voice] transcribe confirmation: %v", err)
			http.Error(w, "transcription failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		out.Decision = classifyAnswer(out.Transcript)
	default:
		http.Error(w, "answer must be yes or no", http.StatusBadRequest)
		return
	}

	if out.Decision == "unclear" {
		if audio, err := h.voice.Speak(r.Context(), "No te entendí. ¿Lo ejecuto, sí o no?"); err != nil {
			log.Printf("[voice] speak reprompt: %v", err)
		} else {
			out.Audio = base64.StdEncoding.EncodeToString(audio)
		}
		writeJSON(w, out)
		return
	}

	if err := h.pending.Resolve(id, voiceUserID, out.Decision == "yes"); err != nil {
		http.Error(w, "this confirmation already expired", http.StatusGone)
		return
	}
	log.Printf("[voice] confirmation %s: %s (%q)", id, out.Decision, out.Transcript)
	writeJSON(w, out)
}

var (
	yesWords = map[string]bool{
		"si": true, "dale": true, "confirmo": true, "confirmado": true, "confirma": true,
		"hazlo": true, "adelante": true, "claro": true, "ok": true, "okay": true, "vale": true,
		"afirmativo": true, "apruebo": true, "aprobado": true, "aprueba": true, "procede": true,
		"ejecutalo": true, "ejecuta": true, "listo": true, "acuerdo": true,
	}
	noWords = map[string]bool{
		"no": true, "cancela": true, "cancelalo": true, "cancelar": true, "para": true,
		"detente": true, "alto": true, "rechazo": true, "rechaza": true, "rechazado": true,
		"negativo": true, "nunca": true, "espera": true, "tampoco": true,
	}
	accents = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u")
)

func classifyAnswer(s string) string {
	words := strings.FieldsFunc(accents.Replace(strings.ToLower(s)), func(r rune) bool {
		return !unicode.IsLetter(r)
	})
	var yes, no bool
	for _, w := range words {
		yes = yes || yesWords[w]
		no = no || noWords[w]
	}
	switch {
	case yes && !no:
		return "yes"
	case no && !yes:
		return "no"
	}
	return "unclear"
}

func spokenName(skill string) string {
	return strings.NewReplacer("_", " ", "-", " ", ".", " ").Replace(skill)
}
