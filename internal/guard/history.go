package guard

import (
	"bufio"
	"encoding/json"
	"os"
	"sort"
	"sync"
	"time"
)

const historySize = 5000

type history struct {
	mu      sync.RWMutex
	entries []Entry
	next    int
	full    bool
	subs    map[chan Entry]struct{}
	closed  bool
}

func newHistory() *history {
	return &history{entries: make([]Entry, historySize), subs: map[chan Entry]struct{}{}}
}

func (h *history) load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			h.push(e)
		}
	}
	return sc.Err()
}

func (h *history) push(e Entry) {
	h.entries[h.next] = e
	h.next = (h.next + 1) % len(h.entries)
	if h.next == 0 {
		h.full = true
	}
}

func (h *history) add(e Entry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.push(e)
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func (h *history) newestFirst() []Entry {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := h.next
	if h.full {
		n = len(h.entries)
	}
	out := make([]Entry, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, h.entries[(h.next-i+len(h.entries))%len(h.entries)])
	}
	return out
}

func (h *history) subscribe() (<-chan Entry, func()) {
	ch := make(chan Entry, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(ch)
		return ch, func() {}
	}
	h.subs[ch] = struct{}{}
	return ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[ch]; ok {
			delete(h.subs, ch)
			close(ch)
		}
	}
}

func (h *history) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for ch := range h.subs {
		delete(h.subs, ch)
		close(ch)
	}
}

type Filter struct {
	Limit    int
	Decision Decision
	Server   string
	Skill    string
	Since    time.Time
}

func (f Filter) match(e Entry) bool {
	return (f.Decision == "" || e.Decision == f.Decision) &&
		(f.Server == "" || e.Server == f.Server) &&
		(f.Skill == "" || e.Skill == f.Skill) &&
		(f.Since.IsZero() || !e.Time.Before(f.Since))
}

func (g *Guard) Recent(f Filter) []Entry {
	out := []Entry{}
	for _, e := range g.hist.newestFirst() {
		if !f.match(e) {
			continue
		}
		out = append(out, e)
		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}
	}
	return out
}

func (g *Guard) Subscribe() (<-chan Entry, func()) {
	return g.hist.subscribe()
}

type GroupStats struct {
	Name          string  `json:"name"`
	Total         int     `json:"total"`
	Allowed       int     `json:"allowed"`
	Blocked       int     `json:"blocked"`
	Errors        int     `json:"errors"`
	AvgDurationMS float64 `json:"avg_duration_ms"`
}

type Stats struct {
	Since         time.Time    `json:"since"`
	Total         int          `json:"total"`
	Allowed       int          `json:"allowed"`
	Blocked       int          `json:"blocked"`
	Denied        int          `json:"denied"`
	Confirm       int          `json:"confirm"`
	Errors        int          `json:"errors"`
	AvgDurationMS float64      `json:"avg_duration_ms"`
	ByServer      []GroupStats `json:"by_server"`
	BySkill       []GroupStats `json:"by_skill"`
}

func (g *Guard) Stats(since time.Time) Stats {
	st := Stats{Since: since, ByServer: []GroupStats{}, BySkill: []GroupStats{}}
	servers := map[string]*groupAcc{}
	skillsAcc := map[string]*groupAcc{}
	var total groupAcc

	for _, e := range g.Recent(Filter{Since: since}) {
		total.add(e)
		server := e.Server
		if server == "" {
			server = "aurora"
		}
		accFor(servers, server).add(e)
		accFor(skillsAcc, e.Skill).add(e)
		switch e.Decision {
		case Deny:
			st.Denied++
		case Confirm:
			st.Confirm++
		}
	}

	t := total.stats("")
	st.Total, st.Allowed, st.Blocked, st.Errors, st.AvgDurationMS = t.Total, t.Allowed, t.Blocked, t.Errors, t.AvgDurationMS
	st.ByServer = groupStats(servers)
	st.BySkill = groupStats(skillsAcc)
	return st
}

type groupAcc struct {
	total, allowed, blocked, errors int
	durMS                           int64
}

func (a *groupAcc) add(e Entry) {
	a.total++
	a.durMS += e.DurationMS
	if e.Decision == Allow {
		a.allowed++
	} else {
		a.blocked++
	}
	if e.Error != "" && e.Decision == Allow {
		a.errors++
	}
}

func (a *groupAcc) stats(name string) GroupStats {
	s := GroupStats{Name: name, Total: a.total, Allowed: a.allowed, Blocked: a.blocked, Errors: a.errors}
	if a.total > 0 {
		s.AvgDurationMS = round(float64(a.durMS) / float64(a.total))
	}
	return s
}

func accFor(m map[string]*groupAcc, k string) *groupAcc {
	a, ok := m[k]
	if !ok {
		a = &groupAcc{}
		m[k] = a
	}
	return a
}

func groupStats(m map[string]*groupAcc) []GroupStats {
	out := make([]GroupStats, 0, len(m))
	for name, a := range m {
		out = append(out, a.stats(name))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].Name < out[j].Name
	})
	return out
}
