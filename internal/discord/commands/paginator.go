package commands

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	pagerSessionTTL  = 15 * time.Minute
	pagerPageSize    = 10
	pagerMaxPageLen  = 3500
	pagerCustomIDFmt = "/pager/%s/%d"
)

type pagerSession struct {
	title     string
	color     int
	pages     []string
	createdAt time.Time
}

type paginator struct {
	mu       sync.Mutex
	sessions map[string]*pagerSession
}

func newPaginator() *paginator {
	return &paginator{sessions: make(map[string]*pagerSession)}
}

func (p *paginator) sendPaginated(e *handler.CommandEvent, title string, color int, lines []string) error {
	return p.sendPages(e, title, color, chunkLines(lines, pagerPageSize, pagerMaxPageLen))
}

func (p *paginator) sendPages(e *handler.CommandEvent, title string, color int, pages []string) error {
	if len(pages) == 0 {
		return e.CreateMessage(discord.NewMessageCreate().
			WithEmbeds(discord.NewEmbed().WithTitle(title).WithColor(color).WithDescription("_Sin contenido._")))
	}
	if len(pages) == 1 {
		return e.CreateMessage(discord.NewMessageCreate().
			WithEmbeds(discord.NewEmbed().WithTitle(title).WithColor(color).WithDescription(pages[0])))
	}

	sessionID := newSessionID()
	p.mu.Lock()
	p.cleanupLocked(time.Now())
	p.sessions[sessionID] = &pagerSession{
		title:     title,
		color:     color,
		pages:     pages,
		createdAt: time.Now(),
	}
	p.mu.Unlock()

	return e.CreateMessage(discord.NewMessageCreate().
		WithEmbeds(p.renderEmbed(sessionID, 0)).
		WithComponents(p.buttons(sessionID, 0, len(pages))))
}

func (p *paginator) renderEmbed(sessionID string, page int) discord.Embed {
	s := p.session(sessionID)
	if s == nil {
		return discord.NewEmbed().
			WithDescription("_La sesión expiró, vuelve a ejecutar el comando._")
	}
	total := len(s.pages)
	embed := discord.NewEmbed().
		WithTitle(s.title).
		WithColor(s.color).
		WithDescription(s.pages[page])
	if total > 1 {
		embed = embed.WithFooterText(fmt.Sprintf("Página %d/%d", page+1, total))
	}
	return embed
}

func (p *paginator) buttons(sessionID string, page, total int) discord.ActionRowComponent {
	prev := discord.NewSecondaryButton("◀", fmt.Sprintf(pagerCustomIDFmt, sessionID, page-1)).
		WithDisabled(page == 0)
	next := discord.NewSecondaryButton("▶", fmt.Sprintf(pagerCustomIDFmt, sessionID, page+1)).
		WithDisabled(page >= total-1)
	return discord.ActionRowComponent{}.WithComponents(prev, next)
}

func (p *paginator) handleButton(e *handler.ComponentEvent) error {
	sessionID := e.Vars["session"]
	pageStr := e.Vars["page"]
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		return err
	}

	s := p.session(sessionID)
	if s == nil {
		return e.UpdateMessage(discord.NewMessageUpdate().
			WithEmbeds(discord.NewEmbed().WithDescription("_La sesión expiró, vuelve a ejecutar el comando._")))
	}

	total := len(s.pages)
	if page < 0 || page >= total {
		return nil
	}

	return e.UpdateMessage(discord.NewMessageUpdate().
		WithEmbeds(p.renderEmbed(sessionID, page)).
		WithComponents(p.buttons(sessionID, page, total)))
}

func (p *paginator) session(sessionID string) *pagerSession {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sessions[sessionID]
}

func (p *paginator) cleanupLocked(now time.Time) {
	for id, s := range p.sessions {
		if now.Sub(s.createdAt) > pagerSessionTTL {
			delete(p.sessions, id)
		}
	}
}

func newSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b)
}

func chunkLines(lines []string, perPage, maxLen int) []string {
	if len(lines) == 0 {
		return nil
	}
	var pages []string
	var page []string
	var pageLen int
	for _, line := range lines {
		if line == "" {
			continue
		}
		if len(page) > 0 && (len(page) >= perPage || pageLen+len(line)+1 > maxLen) {
			pages = append(pages, strings.Join(page, "\n"))
			page = nil
			pageLen = 0
		}
		page = append(page, line)
		pageLen += len(line) + 1
	}
	if len(page) > 0 {
		pages = append(pages, strings.Join(page, "\n"))
	}
	return pages
}
