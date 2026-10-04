package api

import (
	"encoding/json"
	"net/http"

	"github.com/Espectro0/AuroraProject/internal/proposals"
)

type JournalHandler struct {
	journal *proposals.SimpleProcessor
}

func NewJournalHandler(j *proposals.SimpleProcessor) *JournalHandler {
	return &JournalHandler{journal: j}
}

func (h *JournalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	entries, err := h.journal.Read()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
