package api

import (
	"net/http"

	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/proposals"
)

func NewRouter(store memory.MemoryStore, journal *proposals.SimpleProcessor) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/edges", NewEdgesHandler(store))
	mux.Handle("GET /api/v1/journal", NewJournalHandler(journal))
	return mux
}
