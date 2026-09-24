package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/guard"
)

const (
	defaultGuardLimit = 100
	defaultStatsSince = 24 * time.Hour
	sseHeartbeat      = 15 * time.Second
)

type GuardHandler struct {
	guard *guard.Guard
}

func NewGuardHandler(g *guard.Guard) *GuardHandler {
	return &GuardHandler{guard: g}
}

// List serves GET /api/v1/guard?limit=100&decision=confirm&server=github&skill=...&since=24h
func (h *GuardHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := guard.Filter{
		Limit:    defaultGuardLimit,
		Decision: guard.Decision(q.Get("decision")),
		Server:   q.Get("server"),
		Skill:    q.Get("skill"),
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		f.Limit = n
	}
	switch f.Decision {
	case "", guard.Allow, guard.Deny, guard.Confirm:
	default:
		http.Error(w, "invalid decision (allow, deny, confirm)", http.StatusBadRequest)
		return
	}
	if v := q.Get("since"); v != "" {
		since, err := parseSince(v)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.Since = since
	}
	writeJSON(w, h.guard.Recent(f))
}

// Stats serves GET /api/v1/guard/stats?since=24h
func (h *GuardHandler) Stats(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-defaultStatsSince)
	if v := r.URL.Query().Get("since"); v != "" {
		s, err := parseSince(v)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		since = s
	}
	writeJSON(w, h.guard.Stats(since))
}

// Stream serves GET /api/v1/guard/stream as Server-Sent Events
func (h *GuardHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	entries, cancel := h.guard.Subscribe()
	defer cancel()

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	tick := time.NewTicker(sseHeartbeat)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			fmt.Fprint(w, ": ping\n\n")
		case e, ok := <-entries:
			if !ok {
				return
			}
			b, err := json.Marshal(e)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: entry\ndata: %s\n\n", b)
		}
		flusher.Flush()
	}
}

func parseSince(v string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, nil
	}
	if days, ok := strings.CutSuffix(v, "d"); ok {
		n, err := strconv.Atoi(days)
		if err == nil && n >= 0 {
			return time.Now().AddDate(0, 0, -n), nil
		}
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		return time.Time{}, fmt.Errorf("invalid since %q (use 24h, 7d or RFC3339)", v)
	}
	return time.Now().Add(-d), nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
