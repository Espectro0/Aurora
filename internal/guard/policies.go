package guard

import (
	"context"

	"github.com/Espectro0/AuroraProject/config"
)

type ConfirmPolicy struct {
	servers map[string]*config.McpServer
}

func NewConfirmPolicy(cfg *config.McpConfig) *ConfirmPolicy {
	p := &ConfirmPolicy{servers: map[string]*config.McpServer{}}
	if cfg != nil {
		p.servers = cfg.Servers
	}
	return p
}

func (p *ConfirmPolicy) Name() string { return "confirm" }

func (p *ConfirmPolicy) Check(_ context.Context, c Call) Verdict {
	if c.Server == "" {
		return Verdict{Decision: Allow}
	}
	srv, ok := p.servers[c.Server]
	if !ok {
		return Verdict{Decision: Deny, Reason: "servidor MCP desconocido"}
	}
	if srv.NeedsConfirm(c.Tool) {
		return Verdict{Decision: Confirm, Reason: "la tool " + c.Tool + " está en la lista confirm de " + c.Server}
	}
	return Verdict{Decision: Allow}
}
