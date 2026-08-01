package commands

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const statusEmbedColor = 0x2ECC71

func handleStatus(d Deps) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		embed := discord.NewEmbed().
			WithTitle("Status").
			WithColor(statusEmbedColor).
			WithTimestamp(time.Now())

		for _, info := range d.Manager.Info() {
			state := "offline"
			if info.Running {
				state = "running"
			}
			model := filepath.Base(info.Model)
			if model == "." || model == "" {
				model = "—"
			}
			embed = embed.AddField(
				fmt.Sprintf("%s", info.Kind),
				fmt.Sprintf("Status: **%s**\nModel: `%s`\nURL: `%s`", state, model, info.BaseURL),
				false,
			)
		}

		if d.Memory != nil {
			embed = embed.AddField(
				"Mind Graph",
				fmt.Sprintf("Nodes: **%d**\nEdges: **%d**", d.Memory.Count(), len(d.Memory.Edges())),
				false,
			)
		}

		return e.CreateMessage(discord.NewMessageCreate().WithEmbeds(embed))
	}
}
