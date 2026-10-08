package sleep

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/decision"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/skills"
	"github.com/google/uuid"
)

const (
	wikiSearchSkill  = "wikipedia__search_wikipedia"
	wikiArticleSkill = "wikipedia__get_article"
)

const (
	maxSearchResults = 5
	maxArticleChars  = 6000
	noArticle        = "none"
)

type ToolRunner interface {
	Execute(ctx context.Context, name, argsJSON string) (skills.Result, error)
}

type Learner interface {
	Learn(ctx context.Context, topic, curiosity, source, text string) (string, error)
}

func (s *Sleeper) SetExplorer(tools ToolRunner, learner Learner) {
	s.explorerMu.Lock()
	defer s.explorerMu.Unlock()
	s.tools = tools
	s.learner = learner
}

func (s *Sleeper) explore(ctx context.Context) ([]string, error) {
	s.explorerMu.Lock()
	tools, learner := s.tools, s.learner
	s.explorerMu.Unlock()

	cfg := s.identity.Get().Sleep
	if cfg.ExploreDisabled || tools == nil || learner == nil {
		return nil, nil
	}

	nodes, err := s.store.NodesByType(ctx, memory.NodeInterest)
	if err != nil {
		return nil, fmt.Errorf("interests: %w", err)
	}

	now := time.Now()
	cooldown := time.Duration(cfg.ExploreCooldownDays) * 24 * time.Hour
	var candidates []memory.Node
	for _, n := range nodes {
		if n.Owner != "" {
			continue
		}
		if c, _ := n.Metadata[memory.MetaCuriosity].(string); strings.TrimSpace(c) == "" {
			continue
		}
		if at, _ := n.Metadata[memory.MetaExploredAt].(string); at != "" {
			if t, err := time.Parse(time.RFC3339, at); err == nil && now.Sub(t) < cooldown {
				continue
			}
		}
		candidates = append(candidates, n)
	}
	sort.Slice(candidates, func(i, j int) bool { return importanceOf(candidates[i]) > importanceOf(candidates[j]) })
	if len(candidates) > cfg.MaxExplorations {
		candidates = candidates[:cfg.MaxExplorations]
	}

	var explored []string
	for _, n := range candidates {
		if ctx.Err() != nil {
			break
		}
		topic, _ := n.Metadata[memory.MetaTopic].(string)
		if topic == "" {
			topic = n.Content
		}
		curiosity, _ := n.Metadata[memory.MetaCuriosity].(string)

		title, text, err := s.research(ctx, tools, topic, curiosity)
		if err != nil {
			log.Printf("[sleep] explore %q: %v", topic, err)
			continue
		}

		if title != "" {
			reflectionID, err := learner.Learn(ctx, topic, curiosity, "Wikipedia: "+title, text)
			switch {
			case err != nil:
				log.Printf("[sleep] learn %q: %v", topic, err)
				continue
			case reflectionID != "":
				if err := s.store.CreateEdge(ctx, memory.Edge{
					ID:        uuid.New().String(),
					SourceID:  n.ID,
					TargetID:  reflectionID,
					Type:      memory.EdgeLeadsTo,
					Weight:    1,
					CreatedAt: now,
				}); err != nil {
					log.Printf("[sleep] explore edge %s -> %s: %v", n.ID, reflectionID, err)
				}
				explored = append(explored, fmt.Sprintf("%s (%s)", topic, title))
			}
		} else {
			log.Printf("[sleep] explore %q: no matching article", topic)
		}

		if err := s.markExplored(ctx, n, now); err != nil {
			log.Printf("[sleep] mark explored %q: %v", topic, err)
		}
	}
	return explored, nil
}

func (s *Sleeper) markExplored(ctx context.Context, n memory.Node, at time.Time) error {
	meta := make(map[string]any, len(n.Metadata)+1)
	for k, v := range n.Metadata {
		meta[k] = v
	}
	meta[memory.MetaExploredAt] = at.Format(time.RFC3339)
	return s.store.SetMetadata(ctx, n.ID, meta)
}

type wikiSearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

var htmlTags = regexp.MustCompile(`<[^>]+>`)

func (s *Sleeper) research(ctx context.Context, tools ToolRunner, topic, curiosity string) (string, string, error) {
	args, _ := json.Marshal(map[string]any{"query": topic, "limit": maxSearchResults})
	res, err := tools.Execute(ctx, wikiSearchSkill, string(args))
	if err != nil {
		return "", "", fmt.Errorf("search: %w", err)
	}

	var search struct {
		Results []wikiSearchResult `json:"results"`
	}
	if err := json.Unmarshal([]byte(res.Text), &search); err != nil {
		return "", "", fmt.Errorf("search: invalid response: %w", err)
	}
	if len(search.Results) == 0 {
		return "", "", nil
	}

	title := s.pickArticle(ctx, topic, curiosity, search.Results)
	if title == "" {
		return "", "", nil
	}

	args, _ = json.Marshal(map[string]any{"title": title})
	res, err = tools.Execute(ctx, wikiArticleSkill, string(args))
	if err != nil {
		return "", "", fmt.Errorf("article %q: %w", title, err)
	}
	text := articleText(res.Text)
	if text == "" {
		return "", "", nil
	}
	return title, text, nil
}

func (s *Sleeper) pickArticle(ctx context.Context, topic, curiosity string, results []wikiSearchResult) string {
	if s.decision == nil {
		return results[0].Title
	}

	criteria := map[string]string{noArticle: "ningún artículo trata realmente del tema"}
	for i, r := range results {
		criteria[strconv.Itoa(i)] = r.Title + ": " + strings.TrimSpace(htmlTags.ReplaceAllString(r.Snippet, ""))
	}

	answers, err := s.decision.Judge(ctx, fmt.Sprintf("Tema: %s\nCuriosidad: %s", topic, curiosity), map[string]decision.Question{
		"article": {
			Type:         decision.TypeChoice,
			Instructions: "¿Qué artículo de Wikipedia ayuda más a entender este tema y responder esta curiosidad?",
			Criteria:     criteria,
		},
	})
	if err != nil {
		log.Printf("[sleep] pick article decision error, using first result: %v", err)
		return results[0].Title
	}
	ans, ok := answers["article"]
	if !ok || ans.Choice == noArticle {
		return ""
	}
	i, err := strconv.Atoi(ans.Choice)
	if err != nil || i < 0 || i >= len(results) {
		return ""
	}
	return results[i].Title
}

func articleText(raw string) string {
	var article struct {
		Text    string `json:"text"`
		Summary string `json:"summary"`
	}
	text := ""
	if err := json.Unmarshal([]byte(raw), &article); err == nil {
		text = article.Text
		if text == "" {
			text = article.Summary
		}
	} else if idx := strings.Index(raw, `"text":`); idx != -1 {
		rest := strings.TrimSpace(raw[idx+len(`"text":`):])
		rest = strings.TrimPrefix(rest, `"`)
		if end := strings.Index(rest, `", "url"`); end != -1 {
			rest = rest[:end]
		} else if end := strings.Index(rest, `","url"`); end != -1 {
			rest = rest[:end]
		}
		if unq, err := strconv.Unquote(`"` + rest + `"`); err == nil {
			text = unq
		} else {
			text = strings.NewReplacer(`\n`, "\n", `\"`, `"`).Replace(rest)
		}
	}

	r := []rune(strings.TrimSpace(text))
	if len(r) > maxArticleChars {
		r = r[:maxArticleChars]
	}
	return string(r)
}
