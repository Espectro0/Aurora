package sleep

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/decision"
	"github.com/Espectro0/AuroraProject/internal/memory"
)

const defaultImportance = 0.5

const maxMergedFacts = 5

const maxDecayDays = 7

func importanceOf(n memory.Node) float64 {
	if v, ok := n.Importance(); ok {
		return v
	}
	return defaultImportance
}

func (s *Sleeper) mergeDuplicates(ctx context.Context) (int, error) {
	cfg := s.identity.Get()
	groups, err := s.store.FindDuplicates(ctx, cfg.Sleep.MergeThreshold)
	if err != nil {
		return 0, err
	}

	merged := 0
	for _, group := range groups {
		if ctx.Err() != nil {
			return merged, ctx.Err()
		}

		winner, rest := pickWinner(group)
		var losers []memory.Node
		for _, n := range rest {
			if s.sameEntity(ctx, winner, n, cfg.MemoryUsageRules.SameEntityThreshold) {
				losers = append(losers, n)
			}
		}
		if len(losers) == 0 {
			continue
		}

		combined := mergeNodes(winner, losers)
		if err := s.store.UpdateNode(ctx, combined); err != nil {
			log.Printf("[sleep] merge update %s: %v", winner.ID, err)
			continue
		}

		for _, l := range losers {
			if err := s.store.RewireEdges(ctx, l.ID, winner.ID); err != nil {
				log.Printf("[sleep] rewire %s -> %s: %v", l.ID, winner.ID, err)
				continue
			}
			if err := s.store.DeleteNode(ctx, l.ID); err != nil {
				log.Printf("[sleep] delete merged %s: %v", l.ID, err)
				continue
			}
			merged++
			log.Printf("[sleep] merged %q into %q", l.Content, winner.Content)
		}
	}

	return merged, nil
}

func pickWinner(group []memory.Node) (memory.Node, []memory.Node) {
	best := 0
	for i, n := range group[1:] {
		i++
		bi, ni := importanceOf(group[best]), importanceOf(n)
		if ni > bi || (ni == bi && n.CreatedAt.Before(group[best].CreatedAt)) {
			best = i
		}
	}

	rest := make([]memory.Node, 0, len(group)-1)
	for i, n := range group {
		if i != best {
			rest = append(rest, n)
		}
	}
	return group[best], rest
}

func (s *Sleeper) sameEntity(ctx context.Context, a, b memory.Node, threshold float64) bool {
	if s.decision == nil {
		return true
	}

	state := fmt.Sprintf("Recuerdo A: %s\nRecuerdo B: %s", a.Content, b.Content)
	answers, err := s.decision.Judge(ctx, state, map[string]decision.Question{
		"same": {
			Type:         decision.TypeNoul,
			Instructions: "¿Los dos recuerdos hablan de la misma persona, cosa, evento o proyecto (aunque digan hechos distintos sobre ella)?",
		},
	})
	if err != nil {
		log.Printf("[sleep] decision error, not merging %s/%s: %v", a.ID, b.ID, err)
		return false
	}
	ans, ok := answers["same"]
	return ok && ans.Noul >= threshold
}

func splitFacts(content string) (prefix string, facts []string) {
	if idx := strings.Index(content, ": "); idx > 0 {
		return content[:idx], strings.Split(content[idx+2:], "; ")
	}
	return "", []string{content}
}

func mergeNodes(winner memory.Node, losers []memory.Node) memory.Node {
	prefix, facts := splitFacts(winner.Content)
	seen := make(map[string]bool, len(facts))
	for _, f := range facts {
		seen[strings.ToLower(strings.TrimSpace(f))] = true
	}

	meta := make(map[string]any, len(winner.Metadata)+3)
	for k, v := range winner.Metadata {
		meta[k] = v
	}

	importance := importanceOf(winner)
	recalls := winner.RecallCount()
	lastRecalled := winner.LastRecalled()

	for _, l := range losers {
		_, lf := splitFacts(l.Content)
		for _, f := range lf {
			key := strings.ToLower(strings.TrimSpace(f))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			facts = append(facts, f)
		}

		importance = math.Max(importance, importanceOf(l))
		recalls += l.RecallCount()
		if t := l.LastRecalled(); t.After(lastRecalled) {
			lastRecalled = t
		}
		if l.CreatedAt.Before(winner.CreatedAt) && !l.CreatedAt.IsZero() {
			winner.CreatedAt = l.CreatedAt
		}
	}

	if len(facts) > maxMergedFacts {
		facts = facts[len(facts)-maxMergedFacts:]
	}

	meta[memory.MetaImportance] = importance
	meta[memory.MetaUpdatedAt] = time.Now().Format(time.RFC3339)
	if recalls > 0 {
		meta[memory.MetaRecallCount] = recalls
	}
	if !lastRecalled.IsZero() {
		meta[memory.MetaLastRecalledAt] = lastRecalled.Format(time.RFC3339)
	}

	if prefix != "" {
		winner.Content = prefix + ": " + strings.Join(facts, "; ")
	} else {
		winner.Content = strings.Join(facts, "; ")
	}
	winner.Metadata = meta
	winner.Similarity = 0
	return winner
}

func (s *Sleeper) decayAndForget(ctx context.Context, since, now time.Time, report *Report) error {
	cfg := s.identity.Get().Sleep

	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return err
	}

	latestReflection := make(map[string]memory.Node)
	for _, n := range nodes {
		if n.Type == memory.NodeReflection && n.CreatedAt.After(latestReflection[n.Owner].CreatedAt) {
			latestReflection[n.Owner] = n
		}
	}

	elapsed := math.Min(now.Sub(since).Hours()/24, maxDecayDays)
	minAge := time.Duration(cfg.ForgetMinAgeDays) * 24 * time.Hour

	for _, n := range nodes {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		old := importanceOf(n)
		imp := old

		switch {
		case n.LastRecalled().After(since):
			imp += cfg.RecallBoost * (1 - imp)
		case n.LastActive().Before(since) && elapsed > 0:
			halfLife := cfg.DecayHalfLifeDays * (1 + 2*imp)
			imp *= math.Pow(0.5, elapsed/halfLife)
		}
		imp = math.Round(imp*1000) / 1000

		if imp < cfg.ForgetThreshold && now.Sub(n.LastActive()) >= minAge && n.ID != latestReflection[n.Owner].ID {
			if err := s.store.DeleteNode(ctx, n.ID); err != nil {
				log.Printf("[sleep] forget %s: %v", n.ID, err)
				continue
			}
			report.Forgotten = append(report.Forgotten, n.Content)
			log.Printf("[sleep] forgot %q (importance %.3f)", n.Content, imp)
			continue
		}

		if math.Abs(imp-old) < 0.001 {
			continue
		}

		meta := make(map[string]any, len(n.Metadata)+1)
		for k, v := range n.Metadata {
			meta[k] = v
		}
		meta[memory.MetaImportance] = imp
		if err := s.store.SetMetadata(ctx, n.ID, meta); err != nil {
			log.Printf("[sleep] set importance %s: %v", n.ID, err)
			continue
		}

		if imp > old {
			report.Boosted++
		} else {
			report.Decayed++
		}
	}

	return nil
}
