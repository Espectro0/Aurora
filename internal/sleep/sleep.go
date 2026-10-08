package sleep

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Espectro0/AuroraProject/internal/decision"
	"github.com/Espectro0/AuroraProject/internal/identity"
	"github.com/Espectro0/AuroraProject/internal/llm"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/proposals"
)

var ErrAlreadySleeping = errors.New("sleep: ya hay un ciclo de sueño en curso")

type Sleeper struct {
	store     memory.MemoryStore
	llm       llm.Provider
	decision  decision.Provider
	identity  *identity.Core
	journal   *proposals.SimpleProcessor
	statePath string

	running sync.Mutex
	wg      sync.WaitGroup

	lockerMu sync.Mutex
	locker   sync.Locker

	explorerMu sync.Mutex
	tools      ToolRunner
	learner    Learner
}

type Config struct {
	StatePath string
}

func New(store memory.MemoryStore, l llm.Provider, dec decision.Provider, id *identity.Core, journal *proposals.SimpleProcessor, cfg Config) *Sleeper {
	return &Sleeper{
		store:     store,
		llm:       l,
		decision:  dec,
		identity:  id,
		journal:   journal,
		statePath: cfg.StatePath,
	}
}

func (s *Sleeper) SetLocker(l sync.Locker) {
	s.lockerMu.Lock()
	defer s.lockerMu.Unlock()
	s.locker = l
}

type Report struct {
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	Merged      int       `json:"merged"`
	Boosted     int       `json:"boosted"`
	Decayed     int       `json:"decayed"`
	Forgotten   []string  `json:"forgotten"`
	Interests   []string  `json:"interests"`
	Explored    []string  `json:"explored,omitempty"`
	Synthesized []string  `json:"synthesized,omitempty"`
	Errors      []string  `json:"errors,omitempty"`
}

type state struct {
	LastRun    time.Time `json:"last_run"`
	LastReport *Report   `json:"last_report,omitempty"`
}

func (s *Sleeper) Run(ctx context.Context) (Report, error) {
	if !s.running.TryLock() {
		return Report{}, ErrAlreadySleeping
	}
	defer s.running.Unlock()

	s.wg.Add(1)
	defer s.wg.Done()

	s.lockerMu.Lock()
	locker := s.locker
	s.lockerMu.Unlock()
	if locker != nil {
		locker.Lock()
		defer locker.Unlock()
	}

	cfg := s.identity.Get().Sleep
	ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutMinutes)*time.Minute)
	defer cancel()

	st := s.loadState()
	report := Report{StartedAt: time.Now()}
	since := st.LastRun
	if since.IsZero() {
		since = report.StartedAt.Add(-24 * time.Hour)
	}

	log.Printf("[sleep] going to sleep (last run %s)", since.Format(time.RFC3339))

	fail := func(step string, err error) {
		log.Printf("[sleep] %s: %v", step, err)
		report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", step, err))
	}

	if n, err := s.mergeDuplicates(ctx); err != nil {
		fail("merge", err)
	} else {
		report.Merged = n
	}

	if err := s.decayAndForget(ctx, since, report.StartedAt, &report); err != nil {
		fail("decay", err)
	}

	if made, err := s.synthesize(ctx); err != nil {
		fail("synthesis", err)
	} else {
		report.Synthesized = made
	}

	if topics, err := s.regenerateInterests(ctx); err != nil {
		fail("interests", err)
	} else {
		report.Interests = topics
	}

	if explored, err := s.explore(ctx); err != nil {
		fail("explore", err)
	} else {
		report.Explored = explored
	}

	report.FinishedAt = time.Now()

	if ctx.Err() == nil {
		st.LastRun = report.StartedAt
	}
	st.LastReport = &report
	if err := s.saveState(st); err != nil {
		log.Printf("[sleep] save state: %v", err)
	}
	s.writeJournal(report)

	log.Printf("[sleep] woke up: merged=%d boosted=%d decayed=%d forgotten=%d synthesized=%d interests=%d explored=%d errors=%d",
		report.Merged, report.Boosted, report.Decayed, len(report.Forgotten), len(report.Synthesized), len(report.Interests), len(report.Explored), len(report.Errors))

	return report, ctx.Err()
}

func (s *Sleeper) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			next := nextRun(time.Now(), s.identity.Get().Sleep.Hour)
			log.Printf("[sleep] next sleep cycle at %s", next.Format(time.RFC3339))

			timer := time.NewTimer(time.Until(next))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}

			if s.identity.Get().Sleep.Disabled {
				log.Printf("[sleep] nightly cycle disabled, skipping")
				continue
			}
			if _, err := s.Run(ctx); err != nil {
				log.Printf("[sleep] nightly cycle: %v", err)
			}
		}
	}()
}

func (s *Sleeper) Wait() {
	s.wg.Wait()
}

func nextRun(now time.Time, hour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func (s *Sleeper) loadState() state {
	var st state
	if raw, err := os.ReadFile(s.statePath); err == nil {
		_ = json.Unmarshal(raw, &st)
	}
	return st
}

func (s *Sleeper) saveState(st state) error {
	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.statePath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.statePath)
}

const maxJournalForgotten = 5

func (s *Sleeper) writeJournal(r Report) {
	if s.journal == nil {
		return
	}

	var b strings.Builder
	b.WriteString("Dormí y ordené mis recuerdos.")
	if r.Merged > 0 {
		fmt.Fprintf(&b, " Uní %d recuerdos que eran lo mismo.", r.Merged)
	}
	if r.Boosted > 0 {
		fmt.Fprintf(&b, " %d recuerdos se volvieron más fuertes porque los usé.", r.Boosted)
	}
	if r.Decayed > 0 {
		fmt.Fprintf(&b, " %d se fueron desvaneciendo un poco.", r.Decayed)
	}
	if n := len(r.Forgotten); n > 0 {
		shown := r.Forgotten
		if n > maxJournalForgotten {
			shown = shown[:maxJournalForgotten]
		}
		fmt.Fprintf(&b, " Olvidé %d cosas, entre ellas: %s.", n, strings.Join(shown, "; "))
	}
	if len(r.Synthesized) > 0 {
		fmt.Fprintf(&b, " Conecté ideas que no había relacionado: %s.", strings.Join(r.Synthesized, "; "))
	}
	if len(r.Interests) > 0 {
		fmt.Fprintf(&b, " Al despertar me interesa: %s.", strings.Join(r.Interests, ", "))
	}
	if len(r.Explored) > 0 {
		fmt.Fprintf(&b, " Investigué por mi cuenta: %s.", strings.Join(r.Explored, ", "))
	}

	err := s.journal.Process(context.Background(), proposals.Proposal{
		Timestamp: r.FinishedAt,
		Journal:   &proposals.JournalProp{Content: b.String(), Mood: "tranquila"},
	})
	if err != nil {
		log.Printf("[sleep] journal: %v", err)
	}
}
