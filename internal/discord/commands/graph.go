package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	graphEmbedColor  = 0xE67E22
	graphLabelMaxLen = 40
)

func handleGraph(d Deps, pager *paginator) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		if d.Memory == nil {
			return e.CreateMessage(discord.NewMessageCreate().
				WithEmbeds(discord.NewEmbed().WithColor(graphEmbedColor).WithDescription("La memoria no está disponible.")))
		}

		edges := d.Memory.Edges()
		byType := make(map[string]int)
		for _, edge := range edges {
			byType[string(edge.Type)]++
		}

		var lines []string
		lines = append(lines,
			fmt.Sprintf("**Nodos:** %d", d.Memory.Count()),
			fmt.Sprintf("**Aristas:** %d", len(edges)),
		)

		if len(byType) > 0 {
			lines = append(lines, "", "**Aristas por tipo**")
			types := make([]string, 0, len(byType))
			for t := range byType {
				types = append(types, t)
			}
			sort.Strings(types)
			for _, t := range types {
				lines = append(lines, fmt.Sprintf("- %s: %d", t, byType[t]))
			}
		}

		if len(edges) > 0 {
			lines = append(lines, "", "**Últimas conexiones**")
			cache := make(map[string]string)
			for _, edge := range edges {
				src := nodeLabel(e.Ctx, d, edge.SourceID, cache)
				dst := nodeLabel(e.Ctx, d, edge.TargetID, cache)
				lines = append(lines, fmt.Sprintf("- %s [%s] -> %s", src, edge.Type, dst))
			}
		}

		return pager.sendPaginated(e, "Grafo cognitivo", graphEmbedColor, lines)
	}
}

func nodeLabel(ctx context.Context, d Deps, id string, cache map[string]string) string {
	if l, ok := cache[id]; ok {
		return l
	}

	node, err := d.Memory.GetNode(ctx, id)
	if err != nil {
		short := id
		if len(short) > 8 {
			short = short[:8]
		}
		cache[id] = short
		return short
	}

	label := node.Content
	if idx := strings.Index(label, ":"); idx > 0 {
		label = strings.TrimSpace(label[:idx])
	}
	if len(label) > graphLabelMaxLen {
		label = label[:graphLabelMaxLen-3] + "..."
	}
	cache[id] = label
	return label
}
