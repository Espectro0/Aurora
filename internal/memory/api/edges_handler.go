package api

import (
	"net/http"

	"github.com/Espectro0/AuroraProject/internal/memory"
)

type EdgesHandler struct {
	store memory.MemoryStore
}

func NewEdgesHandler(store memory.MemoryStore) *EdgesHandler {
	return &EdgesHandler{
		store: store,
	}
}

func (h *EdgesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, err := h.store.EdgesJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}
