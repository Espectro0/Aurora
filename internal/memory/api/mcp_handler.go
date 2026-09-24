package api

import (
	"net/http"

	"github.com/Espectro0/AuroraProject/internal/mcpclient"
)

type MCPStatus interface {
	Status() []mcpclient.ServerStatus
}

type MCPHandler struct {
	mcp MCPStatus
}

func NewMCPHandler(m MCPStatus) *MCPHandler {
	return &MCPHandler{mcp: m}
}

func (h *MCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	servers := []mcpclient.ServerStatus{}
	if h.mcp != nil {
		servers = h.mcp.Status()
	}
	writeJSON(w, servers)
}
