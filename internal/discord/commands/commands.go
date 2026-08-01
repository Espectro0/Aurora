package commands

import (
	"github.com/Espectro0/AuroraProject/internal/identity"
	"github.com/Espectro0/AuroraProject/internal/llm"
	"github.com/Espectro0/AuroraProject/internal/localai"
	"github.com/Espectro0/AuroraProject/internal/memory"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

type Deps struct {
	Identity    *identity.Core
	Memory      memory.MemoryStore
	Manager     *localai.ServerManager
	Chat        llm.Provider
	JournalPath string
}

type Commands struct {
	mux  *handler.Mux
	defs []discord.ApplicationCommandCreate
}

func New(d Deps) *Commands {
	mux := handler.New()
	pager := newPaginator()

	mux.Command("/who", handleWho(d))
	mux.Command("/status", handleStatus(d))
	mux.Command("/journal", handleJournal(d, pager))
	mux.Command("/graph", handleGraph(d, pager))
	mux.Component("/pager/{session}/{page}", pager.handleButton)

	return &Commands{
		mux: mux,
		defs: []discord.ApplicationCommandCreate{
			discord.SlashCommandCreate{Name: "who", Description: "Quién es Aurora: identidad, valores y propósito"},
			discord.SlashCommandCreate{Name: "status", Description: "Estado de Aurora: modelos, servidores y memoria"},
			discord.SlashCommandCreate{Name: "journal", Description: "Últimas entradas del diario de Aurora"},
			discord.SlashCommandCreate{Name: "graph", Description: "Resumen del grafo cognitivo de Aurora"},
		},
	}
}

func (c *Commands) Mux() *handler.Mux {
	return c.mux
}

func (c *Commands) Definitions() []discord.ApplicationCommandCreate {
	return c.defs
}
