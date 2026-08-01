package commands

import (
	"os"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	journalTailLines    = 30
	journalLinesPerPage = 10
	journalEmbedColor   = 0x3498DB
)

func handleJournal(d Deps, pager *paginator) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		content, err := tailFile(d.JournalPath, journalTailLines)
		if err != nil {
			return e.CreateMessage(discord.NewMessageCreate().
				WithEmbeds(discord.NewEmbed().WithColor(journalEmbedColor).WithDescription("No encontré el diario de Aurora.")))
		}
		if strings.TrimSpace(content) == "" {
			return e.CreateMessage(discord.NewMessageCreate().
				WithEmbeds(discord.NewEmbed().WithColor(journalEmbedColor).WithDescription("El diario de Aurora está vacío por ahora.")))
		}

		lines := strings.Split(content, "\n")
		pages := make([]string, 0, (len(lines)+journalLinesPerPage-1)/journalLinesPerPage)
		for i := 0; i < len(lines); i += journalLinesPerPage {
			end := i + journalLinesPerPage
			if end > len(lines) {
				end = len(lines)
			}
			pages = append(pages, "```\n"+strings.Join(lines[i:end], "\n")+"\n```")
		}

		return pager.sendPages(e, "Diario de Aurora", journalEmbedColor, pages)
	}
}

func tailFile(path string, lines int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	parts := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(parts) <= lines {
		return strings.TrimSpace(string(data)), nil
	}

	return strings.Join(parts[len(parts)-lines:], "\n"), nil
}
