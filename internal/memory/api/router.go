package api

import (
	"net/http"

	"github.com/Espectro0/AuroraProject/internal/guard"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/proposals"
)

func NewRouter(store memory.MemoryStore, journal *proposals.SimpleProcessor, g *guard.Guard, mcp MCPStatus, voice *VoiceHandler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/edges", NewEdgesHandler(store))
	mux.Handle("GET /api/v1/journal", NewJournalHandler(journal))

	gh := NewGuardHandler(g)
	mux.HandleFunc("GET /api/v1/guard", gh.List)
	mux.HandleFunc("GET /api/v1/guard/stats", gh.Stats)
	mux.HandleFunc("GET /api/v1/guard/stream", gh.Stream)

	mux.Handle("GET /api/v1/mcp/servers", NewMCPHandler(mcp))

	if voice != nil {
		mux.Handle("POST /api/v1/voice", voice)
	}
	return mux
}
