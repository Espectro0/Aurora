package api

import (
	"net/http"

	"github.com/Espectro0/AuroraProject/internal/memory"
)

func NewRouter(store memory.MemoryStore) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /edges", NewEdgesHandler(store))
	return mux
}
